package testing

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/waterfall/client"
	"github.com/waterfall/net/qemu"
	waterfall_grpc "github.com/waterfall/proto/waterfall_go_grpc"
	"github.com/waterfall/testutils"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
)

var (
	// test args
	launcher = flag.String("launcher", "", "The path to the emulator launcher")
	adbTurbo = flag.String("adb_turbo", "", "The path to abd.turbo binary")
	server   = flag.String("server", "", "The path to test server")

	contents             = testBytes(16 * 1024)
	modeDir  os.FileMode = 0755

	testDir = fs{
		path:  "zoo",
		files: []string{"a.txt", "b.txt"},
		dirs: []fs{
			fs{
				path: "zoo/bar",
			},
			fs{
				path:  "zoo/baz",
				files: []string{"d.txt"},
				dirs: []fs{
					fs{
						path:  "zoo/baz/qux",
						files: []string{"e.txt"},
					}}}}}
)

type fs struct {
	path  string
	files []string
	dirs  []fs
}

const emuWorkingDir = "images/session"
const socketName = "sockets/h2o"

func init() {
	flag.Parse()

	if *launcher == "" || *adbTurbo == "" || *server == "" {
		log.Fatalf("launcher, adb and server args need to be provided")
	}
}

func testBytes(size uint32) []byte {
	bb := new(bytes.Buffer)
	var i uint32
	for i = 0; i < size; i += 4 {
		binary.Write(bb, binary.LittleEndian, i)
	}
	return bb.Bytes()
}

func makeTestFile(path string, bits []byte) error {
	return ioutil.WriteFile(path, bits, 0655)
}

func makeFs(baseDir string, tree fs) ([]string, error) {
	seen := []string{}
	dirPath := filepath.Join(baseDir, tree.path)
	if tree.path != "" {
		seen = append(seen, tree.path)
		if err := os.Mkdir(dirPath, modeDir); err != nil {
			return nil, err
		}
	}
	for _, f := range tree.files {
		seen = append(seen, filepath.Join(tree.path, f))
		if err := makeTestFile(filepath.Join(dirPath, f), contents); err != nil {
			return nil, err
		}
	}
	for _, nfs := range tree.dirs {
		s, err := makeFs(baseDir, nfs)
		if err != nil {
			return nil, err
		}
		seen = append(seen, s...)
	}
	return seen, nil
}

func dirCompare(dir1, dir2, path string, info os.FileInfo, seen map[string]bool) error {
	if path == dir1 {
		return nil
	}

	rel := path[len(dir1)+1:]
	seen[rel] = true
	if info.IsDir() {
		return nil
	}

	rf := filepath.Join(dir2, rel)
	if _, err := os.Stat(rf); os.IsNotExist(err) {
		return err
	}

	bs, err := ioutil.ReadFile(rf)
	if err != nil {
		return err
	}

	if bytes.Compare(contents, bs) != 0 {
		return fmt.Errorf("bytes for file %s not as expected", rel)
	}
	return nil
}

func runServer(ctx context.Context, adbTurbo, adbPort, server string) error {
	s := "localhost:" + adbPort
	_, err := testutils.ExecOnDevice(
		ctx, adbTurbo, s, "push", []string{server, "/data/local/tmp/server"})
	if err != nil {
		return err
	}

	_, err = testutils.ExecOnDevice(
		ctx, adbTurbo, s, "shell", []string{"chmod", "+x", "/data/local/tmp/server"})
	if err != nil {
		return err
	}
	go func() {
		testutils.ExecOnDevice(
			ctx, adbTurbo, s, "shell", []string{"/data/local/tmp/server"})
	}()
	return nil
}

// TestConnection tests that the bytes between device and host are sent/received correctly
func TestConnection(t *testing.T) {
	adbServerPort, adbPort, emuPort, err := testutils.GetAdbPorts()
	if err != nil {
		t.Fatal(err)
	}

	l := filepath.Join(testutils.RunfilesRoot(), *launcher)
	a := filepath.Join(testutils.RunfilesRoot(), *adbTurbo)
	svr := filepath.Join(testutils.RunfilesRoot(), *server)

	emuDir, err := testutils.SetupEmu(l, adbServerPort, adbPort, emuPort)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(emuDir)
	defer testutils.KillEmu(l, adbServerPort, adbPort, emuPort)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	if err := runServer(ctx, a, adbPort, svr); err != nil {
		t.Fatal(err)
	}

	cb, err := qemu.MakeConnBuilder(filepath.Join(emuDir, emuWorkingDir), socketName)
	if err != nil {
		t.Fatalf("error opening qemu connection: %v", err)
	}
	defer cb.Close()

	// Test a few parallel connections
	eg, ctx := errgroup.WithContext(ctx)
	for i := 0; i < 16; i++ {
		eg.Go(func() error {

			qconn, err := cb.Accept()
			if err != nil {
				t.Fatal(err)
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
			rec, err := client.Echo(ctx, k, sent)

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

func TestPushPull(t *testing.T) {
	adbServerPort, adbPort, emuPort, err := testutils.GetAdbPorts()
	if err != nil {
		t.Fatal(err)
	}

	l := filepath.Join(testutils.RunfilesRoot(), *launcher)
	a := filepath.Join(testutils.RunfilesRoot(), *adbTurbo)
	svr := filepath.Join(testutils.RunfilesRoot(), *server)

	emuDir, err := testutils.SetupEmu(l, adbServerPort, adbPort, emuPort)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(emuDir)
	defer testutils.KillEmu(l, adbServerPort, adbPort, emuPort)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	if err := runServer(ctx, a, adbPort, svr); err != nil {
		t.Fatal(err)
	}

	cb, err := qemu.MakeConnBuilder(filepath.Join(emuDir, emuWorkingDir), socketName)
	if err != nil {
		t.Fatalf("error opening qemu connection: %v", err)
	}
	defer cb.Close()

	qconn, err := cb.Accept()
	if err != nil {
		t.Fatal(err)
	}
	defer qconn.Close()

	opts := []grpc.DialOption{grpc.WithInsecure()}
	opts = append(opts, grpc.WithDialer(func(addr string, d time.Duration) (net.Conn, error) {
		return qconn, nil
	}))
	conn, err := grpc.Dial("", opts...)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	td, err := ioutil.TempDir("", "push")
	if err != nil {
		t.Fatal(err)
	}

	pushed, err := makeFs(td, testDir)
	if err != nil {
		t.Fatal(err)
	}

	k := waterfall_grpc.NewWaterfallClient(conn)
	deviceDir := "/data/local/tmp/zoo"
	if err := client.Push(ctx, k, filepath.Join(td, testDir.path), deviceDir); err != nil {
		t.Fatalf("failed push: %v", err)
	}

	pd, err := ioutil.TempDir("", "pull")
	if err != nil {
		t.Fatal(err)
	}

	if err := client.Pull(ctx, k, deviceDir, pd); err != nil {
		t.Fatalf("failed pull: %v", err)
	}

	seen := make(map[string]bool)
	err = filepath.Walk(td, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		return dirCompare(td, pd, path, info, seen)
	})

	if err != nil {
		t.Errorf("pushed != pulled: %v", err)
	}

	if len(seen) != len(pushed) {
		t.Errorf("wrong number of files. Got %v expected %v", seen, pushed)
	}

	for _, f := range pushed {
		if !seen[f] {
			t.Errorf("file %s not in tared files", f)
		}
	}
}

func TestExec(t *testing.T) {
	adbServerPort, adbPort, emuPort, err := testutils.GetAdbPorts()
	if err != nil {
		t.Fatal(err)
	}

	l := filepath.Join(testutils.RunfilesRoot(), *launcher)
	a := filepath.Join(testutils.RunfilesRoot(), *adbTurbo)
	svr := filepath.Join(testutils.RunfilesRoot(), *server)

	emuDir, err := testutils.SetupEmu(l, adbServerPort, adbPort, emuPort)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(emuDir)
	defer testutils.KillEmu(l, adbServerPort, adbPort, emuPort)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	if err := runServer(ctx, a, adbPort, svr); err != nil {
		t.Fatal(err)
	}

	cb, err := qemu.MakeConnBuilder(filepath.Join(emuDir, emuWorkingDir), socketName)
	if err != nil {
		t.Fatalf("error opening qemu connection: %v", err)
	}
	defer cb.Close()

	qconn, err := cb.Accept()
	if err != nil {
		t.Fatal(err)
	}
	defer qconn.Close()

	opts := []grpc.DialOption{grpc.WithInsecure()}
	opts = append(opts, grpc.WithDialer(func(addr string, d time.Duration) (net.Conn, error) {
		return qconn, nil
	}))
	conn, err := grpc.Dial("", opts...)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	k := waterfall_grpc.NewWaterfallClient(conn)

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	r, err := client.Exec(ctx, k, stdout, stderr, nil, "setprop", "ninjas.h20.running", "yes")
	if err != nil {
		t.Fatal(err)
	}
	if r != 0 {
		t.Errorf("setprop command returned non-zero status: %d", r)
		return
	}

	stdout = new(bytes.Buffer)
	stderr = new(bytes.Buffer)
	r, err = client.Exec(ctx, k, stdout, stderr, nil, "getprop", "ninjas.h20.running")
	if err != nil {
		t.Fatal(err)
	}
	if r != 0 {
		t.Errorf("getprop command returned non-zero status: %d", r)
		return
	}

	expected := []byte{'y', 'e', 's', '\n'}
	if bytes.Compare(expected, stdout.Bytes()) != 0 {
		t.Errorf("expected bytes %v but got %v", expected, stdout)
	}

	stdout = new(bytes.Buffer)
	stderr = new(bytes.Buffer)
	r, err = client.Exec(ctx, k, stdout, stderr, nil, "true")
	if err != nil {
		t.Fatal(err)
	}
	if r != 0 {
		t.Errorf("true command returned non-zero status: %d", r)
		return
	}

	stdout = new(bytes.Buffer)
	stderr = new(bytes.Buffer)
	r, err = client.Exec(ctx, k, stdout, stderr, nil, "false")
	if err != nil {
		t.Fatal(err)
	}
	if r == 0 {
		t.Errorf("expected non-zero exit code but got %d", r)
	}
}

func TestExecPipeIn(t *testing.T) {
	adbServerPort, adbPort, emuPort, err := testutils.GetAdbPorts()
	if err != nil {
		t.Fatal(err)
	}

	l := filepath.Join(testutils.RunfilesRoot(), *launcher)
	a := filepath.Join(testutils.RunfilesRoot(), *adbTurbo)
	svr := filepath.Join(testutils.RunfilesRoot(), *server)

	emuDir, err := testutils.SetupEmu(l, adbServerPort, adbPort, emuPort)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(emuDir)
	defer testutils.KillEmu(l, adbServerPort, adbPort, emuPort)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	if err := runServer(ctx, a, adbPort, svr); err != nil {
		t.Fatal(err)
	}

	cb, err := qemu.MakeConnBuilder(filepath.Join(emuDir, emuWorkingDir), socketName)
	if err != nil {
		t.Fatalf("error opening qemu connection: %v", err)
	}
	defer cb.Close()

	qconn, err := cb.Accept()
	if err != nil {
		t.Fatal(err)
	}
	defer qconn.Close()

	opts := []grpc.DialOption{grpc.WithInsecure()}
	opts = append(opts, grpc.WithDialer(func(addr string, d time.Duration) (net.Conn, error) {
		return qconn, nil
	}))
	conn, err := grpc.Dial("", opts...)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	k := waterfall_grpc.NewWaterfallClient(conn)

	st := []string{"foo", "bar", "qux", "baz", "broom", "zoo", "tux", "12", "rust", "fox", "fux"}

	r, w := io.Pipe()
	go func() {
		for _, l := range st {
			w.Write([]byte(fmt.Sprintf("%s\n", l)))
		}
		w.Close()
	}()

	stdout := new(bytes.Buffer)
	ret, err := client.Exec(ctx, k, stdout, ioutil.Discard, r, "sort")
	if err != nil {
		t.Fatal(err)
	}
	if ret != 0 {
		t.Errorf("sort command returned non-zero status: %d", ret)
		return
	}

	sort.Strings(st)
	res := strings.Split(stdout.String(), "\n")

	for i, s := range st {
		if s != res[i] {
			t.Errorf("got unexpected output %s != %s", s, res[i])
			break
		}
	}
}
