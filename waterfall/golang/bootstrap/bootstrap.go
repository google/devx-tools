// Copyright 2018 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package bootstrap bootstraps waterfall using adb.
package bootstrap

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/waterfall/golang/adb"
	"github.com/google/waterfall/golang/client"
	"github.com/google/waterfall/golang/net/qemu"
	waterfall_grpc_pb "github.com/google/waterfall/proto/waterfall_go_grpc"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
)

const (
	helloMsg            = "hello"
	waterfallBinPath    = "/data/local/tmp/waterfall"
	waterfallScriptPath = "/data/local/tmp/waterfall.sh"

	waterfallSh = `#!/system/bin/sh
# start waterfall through adb
# but make sure it does not die if adb dies
waterfall_bin=%s
addr=%s
cd /data/local/tmp
# prefer root if possible
su -c nohup "${waterfall_bin}" --addr "${addr}" || nohup "${waterfall_bin}" --addr "${addr}"`
)

var defaultTimeout = 300 * time.Millisecond

// Result has information about a successful bootstrap session.
type Result struct {
	StartedServer    bool
	StartedForwarder bool
}

func retry(fn func() error, attempts int) error {
	var err error
	for i := 0; i < attempts; i++ {
		err = fn()
		if err == nil {
			return nil
		}
		time.Sleep(defaultTimeout)
	}
	return err
}

func pingServer(ctx context.Context, addr string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cc, err := grpc.DialContext(ctx, addr, grpc.WithInsecure())
	if err != nil {
		return err
	}

	wc := waterfall_grpc_pb.NewWaterfallClient(cc)
	r, err := client.Echo(ctx, wc, []byte(helloMsg))
	if err != nil {
		return err
	}

	if string(r) != helloMsg {
		return fmt.Errorf("unexpected response message: %s", string(r))
	}
	return nil
}

func connectToPipe(ctx context.Context, socketDir, socketName string) error {
	qb, err := qemu.MakeConnBuilder(socketDir, socketName)
	if err != nil {
		return err
	}
	defer qb.Close()

	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	eg, _ := errgroup.WithContext(ctx)

	// If the server is not running this will block waiting for a connection.
	eg.Go(func() error {
		c, err := qb.Accept()
		if err != nil {
			return nil
		}
		c.Close()
		return nil
	})
	return eg.Wait()
}

func startWaterfall(adbConn *adb.Device, wtf, addr string) error {
	if err := adbConn.Push(wtf, waterfallBinPath); err != nil {
		return err
	}

	sc := fmt.Sprintf(waterfallSh, waterfallBinPath, addr)
	dir, err := ioutil.TempDir("", "waterfall")
	defer os.RemoveAll(dir)

	name := filepath.Join(dir, "waterfall.sh")
	if err := ioutil.WriteFile(name, []byte(sc), 0766); err != nil {
		return err
	}

	if err := adbConn.Push(name, waterfallScriptPath); err != nil {
		return err
	}

	err = adbConn.StartCmd(waterfallScriptPath)
	return err
}

func startForwarder(forwarderBin, lisAddr, connAddr string) error {
	log.Println("Starting forwarder server ...")

	// fork a new process with the right params.
	cmd := exec.Command(forwarderBin, "-connect_addr", connAddr, "-listen_addr", lisAddr)

	if err := cmd.Start(); err != nil {
		return err
	}
	cmd.Process.Release()
	return nil
}

func waterfallPath(adbConn *adb.Device, paths []string) (string, error) {
	abis, err := adbConn.AbiList()
	if err != nil {
		return "", err
	}

	archPaths := make(map[string]string)
	for _, p := range paths {
		pts := strings.Split(p, ":")
		if len(pts) != 2 {
			return "", fmt.Errorf("can't parse server path %s", p)
		}
		archPaths[pts[0]] = pts[1]
	}

	for _, abi := range strings.Split(abis, ",") {
		if p, ok := archPaths[abi]; ok {
			return p, nil
		}
	}
	return "", fmt.Errorf("no valid server for architectures %s got %v", abis, paths)
}

// Bootstrap installs h2o on the device and starts the host forwarder if needed.
func Bootstrap(ctx context.Context, adbConn *adb.Device, waterfallBin []string, forwarderBin, socketDir string, socketName string, connectionTimeout int) (*Result, error) {
	// The default bootstrap address.
	unixName := fmt.Sprintf("h2o_%s", adbConn.DeviceName)
	lisAddr := fmt.Sprintf("unix:@%s", unixName)
	timeout := time.Duration(connectionTimeout) * time.Millisecond

	if err := pingServer(ctx, lisAddr, timeout); err == nil {
		// There is responsive forwarder server already running
		return &Result{}, nil
	}

	svr, err := waterfallPath(adbConn, waterfallBin)
	if err != nil {
		return nil, err
	}

	ss := false
	sf := false
	if socketDir != "" {
		// has qemu pipe support
		connAddr := fmt.Sprintf("qemu:%s:%s", socketDir, socketName)
		svrAddr := fmt.Sprintf("qemu-guest:%s", socketName)

		if _, err := os.Stat(filepath.Join(socketDir, socketName)); os.IsNotExist(err) {
			// Forwarder is not running.
			if err := startForwarder(forwarderBin, lisAddr, connAddr); err != nil {
				return nil, err
			}
			sf = true
		}

		if err := retry(func() error {
			return pingServer(ctx, lisAddr, timeout)
		}, 3); err == nil {
			// There is responsive waterfall server already running
			return &Result{StartedServer: ss, StartedForwarder: sf}, nil
		}

		if err := startWaterfall(adbConn, svr, svrAddr); err != nil {
			return nil, err
		}
		ss = true
	} else {
		// will tunnel through adb
		if err := adbConn.ForwardAbstract(unixName, unixName); err != nil {
			return nil, fmt.Errorf("error forwarding through adb: %v", err)
		}

		if err := pingServer(ctx, lisAddr, timeout); err != nil {
			if err := startWaterfall(adbConn, svr, lisAddr); err != nil {
				return nil, err
			}
			ss = true
			sf = false
		}
	}

	// Verify everything was set up correctly before confirming to client
	if err := retry(func() error {
		return pingServer(ctx, lisAddr, timeout)
	}, 3); err != nil {
		return nil, err
	}
	return &Result{StartedServer: ss, StartedForwarder: sf}, nil
}
