package testing

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"flag"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/waterfall/net/qemu"
	waterfall_grpc "github.com/waterfall/proto/waterfall_go_grpc"
	"github.com/waterfall/test_utils"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
)

var (
	runfiles string

	// test args
	launcher = flag.String("launcher", "", "The path to the emulator launcher")
	adbTurbo = flag.String("adb_turbo", "", "The path to abd.turbo binary")
	server   = flag.String("server", "", "The path to test server")
)

const emuWorkingDir = "images/session"

func init() {
	flag.Parse()

	if *launcher == "" || *adbTurbo == "" || *server == "" {
		log.Fatalf("launcher, adb and server args need to be provided")
	}

	wd, err := os.Getwd()
	if err != nil {
		panic("unable to get wd")
	}
	runfiles = runfilesRoot(wd)
}

func runfilesRoot(path string) string {
	sep := "qemu_server_test.runfiles/__main__"
	return path[0 : strings.LastIndex(path, sep)+len(sep)]
}

func testBytes(size uint32) []byte {
	bb := new(bytes.Buffer)
	var i uint32
	for i = 0; i < size; i += 4 {
		binary.Write(bb, binary.LittleEndian, i)
	}
	return bb.Bytes()
}

func runServer(ctx context.Context, adbTurbo, adbPort, server string) error {
	s := "localhost:" + adbPort
	_, err := test_utils.ExecOnDevice(
		ctx, adbTurbo, s, "push", []string{server, "/data/local/tmp/server"})
	if err != nil {
		return err
	}

	_, err = test_utils.ExecOnDevice(
		ctx, adbTurbo, s, "shell", []string{"chmod", "+x", "/data/local/tmp/server"})
	if err != nil {
		return err
	}
	go func() {
		test_utils.ExecOnDevice(
			ctx, adbTurbo, s, "shell", []string{"/data/local/tmp/server"})
	}()
	return nil
}

// TestConnection tests that the bytes between device and host are sent/received correctly
func TestConnection(t *testing.T) {
	adbServerPort, adbPort, emuPort, err := test_utils.GetAdbPorts()
	if err != nil {
		t.Fatal(err)
	}

	l := filepath.Join(runfiles, *launcher)
	a := filepath.Join(runfiles, *adbTurbo)
	svr := filepath.Join(runfiles, *server)

	emuDir, err := test_utils.SetupEmu(l, adbServerPort, adbPort, emuPort)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(emuDir)
	defer test_utils.KillEmu(l, adbServerPort, adbPort, emuPort)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	if err := runServer(ctx, a, adbPort, svr); err != nil {
		t.Fatal(err)
	}

	lis, err := test_utils.OpenSocket(filepath.Join(emuDir, emuWorkingDir), qemu.SocketName)
	if err != nil {
		t.Fatalf("error opening socket: %v", err)
	}
	defer lis.Close()

	// Test a few parallel connections
	eg, ctx := errgroup.WithContext(ctx)
	for i := 0; i < 16; i++ {
		eg.Go(func() error {
			qconn, err := qemu.MakeConn(lis)
			if err != nil {
				return err
			}
			defer qconn.Close()

			opts := []grpc.DialOption{grpc.WithInsecure()}
			opts = append(opts, grpc.WithDialer(
				func(addr string, d time.Duration) (net.Conn, error) {
					return qconn, nil
				}))

			conn, err := grpc.Dial("", opts...)
			if err != nil {
				return err
			}
			defer conn.Close()

			k := waterfall_grpc.NewWaterfallClient(conn)
			sent := testBytes(64 * 1024 * 1024)
			rec, err := Echo(ctx, k, sent)

			if err != nil {
				return err
			}

			if bytes.Compare(sent, rec) != 0 {
				return errors.New("bytes received != bytes sent")
			}
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		t.Fatalf("failed with error: %v", err)
	}
}
