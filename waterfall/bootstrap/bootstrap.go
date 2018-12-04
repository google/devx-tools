// Package bootstrap bootstraps waterfall using adb.
package bootstrap

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/waterfall/adb"
	"github.com/waterfall/client"
	"github.com/waterfall/net/qemu"
	"google.golang.org/grpc"

	waterfall_grpc "github.com/waterfall/proto/waterfall_go_grpc"
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

// Result has information about a succesful bootstrap session.
type Result struct {
	StartedServer    bool
	StartedForwarder bool
}

func retry(fn func() error, attempts int, delay time.Duration) error {
	var err error
	for i := 0; i < attempts; i++ {
		err = fn()
		if err == nil {
			return nil
		}
		time.Sleep(delay)
	}
	return err
}

func withTimeout(f func() error, t time.Duration) error {
	rCh := make(chan error, 1)
	go func() {
		rCh <- f()
	}()

	select {
	case err := <-rCh:
		return err
	case <-time.After(t):
		return fmt.Errorf("timed out")
	}

}

func pingServer(addr string) error {
	return withTimeout(func() error {
		cc, err := grpc.Dial(addr, grpc.WithInsecure())
		if err != nil {
			return err
		}

		wc := waterfall_grpc.NewWaterfallClient(cc)
		resp, err := client.Echo(context.Background(), wc, []byte(helloMsg))
		if err != nil {
			return err
		}

		if string(resp) != helloMsg {
			return fmt.Errorf("unexpected response message: %s", string(resp))
		}
		return nil
	}, defaultTimeout)
}

func connectToPipe(socketDir, socketName string) error {
	qb, err := qemu.MakeConnBuilder(socketDir, socketName)
	if err != nil {
		return err
	}
	defer qb.Close()

	connCh := make(chan net.Conn, 1)
	errCh := make(chan error, 1)

	// If the server is not running this will block waiting for a connection.
	go func() {
		c, err := qb.Accept()
		if err != nil {
			errCh <- err
			return
		}
		c.Close()
		connCh <- c
	}()

	select {
	case <-connCh:
		return nil
	case err := <-errCh:
		return err
	case <-time.After(time.Millisecond * 300):
		return fmt.Errorf("timed out")
	}

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
func Bootstrap(adbConn *adb.Device, waterfallBin []string, forwarderBin, socketDir string, socketName string) (*Result, error) {
	// The default bootstrap address.
	unixName := fmt.Sprintf("h2o_%s", adbConn.DeviceName)
	lisAddr := fmt.Sprintf("unix:@%s", unixName)

	log.Println("Pinging server ...")
	if err := pingServer(lisAddr); err == nil {
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
		svrAddr := fmt.Sprintf("qemu:%s", socketName)

		// Start the server. If already running this wll be a no-op.
		if err := startWaterfall(adbConn, svr, svrAddr); err != nil {
			return nil, err
		}
		ss = true

		if err := retry(func() error {
			return pingServer(lisAddr)
		}, 3, defaultTimeout); err == nil {
			// There is responsive forwarder server already running
			return &Result{StartedServer: ss, StartedForwarder: false}, nil
		}
		if err := startForwarder(forwarderBin, lisAddr, connAddr); err != nil {
			return nil, err
		}
		sf = true
	} else {
		// will tunnel through adb
		if err := adbConn.ForwardAbstract(unixName, unixName); err != nil {
			return nil, fmt.Errorf("error forwarding through adb: %v", err)
		}

		if err := pingServer(lisAddr); err != nil {
			if err := startWaterfall(adbConn, svr, lisAddr); err != nil {
				return nil, err
			}
			ss = true
			sf = false
		}

	}

	// Verify everything was set up correctly before confirming to client
	if err := retry(func() error { return pingServer(lisAddr) }, 3, defaultTimeout); err != nil {
		return nil, err
	}
	return &Result{StartedServer: ss, StartedForwarder: sf}, nil
}
