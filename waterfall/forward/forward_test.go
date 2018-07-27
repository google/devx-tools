package forward

import (
	"flag"
	"hash/fnv"
	"io"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/waterfall/testutils"
)

// Tests in this package perform integration tests with the forward binary

var (
	runfiles string

	fwdr = flag.String("fwdr", "", "The path to forwarder server")
)

func init() {
	flag.Parse()

	wd, err := os.Getwd()
	if err != nil {
		panic("unable to get wd")
	}
	runfiles = runfilesRoot(wd)
}

func runfilesRoot(path string) string {
	sep := "forward_test.runfiles/__main__"
	return path[0 : strings.LastIndex(path, sep)+len(sep)]
}

type eofReader struct {
	net.Conn
	seenEOF bool
}

func (e *eofReader) Read(b []byte) (int, error) {
	if e.seenEOF {
		return 0, io.EOF
	}
	n, err := e.Conn.Read(b)
	if err != nil {
		return 0, err
	}
	for _, bte := range b[:n] {
		// 255 signals the EOF this way we can test the connection is closed
		// from the client side.
		if bte == 255 {
			e.seenEOF = true
			return n - 1, nil
		}
	}
	return n, nil
}

func runCloseableEcho(lis net.Listener, errCh chan error) {
	// connections accepted here will echo back the received stream until they see
	// the special value 255.
	for {
		conn, err := lis.Accept()
		if err != nil {
			errCh <- err
			return
		}
		defer conn.Close()

		cc := &eofReader{Conn: conn}
		go func() {
			defer conn.Close()
			if _, err = io.Copy(conn, cc); err != nil {
				errCh <- err
			}
		}()
	}
}

func runFwdr(path string, lisOn, fwdTo string) error {
	cmd := exec.Command(
		path, "-listen_addr", "tcp:localhost:"+lisOn, "-connect_addr", "tcp:localhost:"+fwdTo)
	return cmd.Start()
}

// TestForward starts the forward server and verifies connection forwarding
func TestForward(t *testing.T) {
	sp, err := testutils.PickUnusedPort()
	if err != nil {
		t.Fatalf("Unable to pick a free port %v", err)
	}
	svrPort := strconv.Itoa(sp)

	svrLis, err := net.Listen("tcp", "localhost:"+svrPort)
	if err != nil {
		t.Fatalf("Unable to bind to port %v", err)
	}
	defer svrLis.Close()

	fp, err := testutils.PickUnusedPort()
	if err != nil {
		t.Fatalf("Unable to pick a free port %v", err)
	}
	fwdrPort := strconv.Itoa(fp)

	ec := make(chan error, 3)
	go runCloseableEcho(svrLis, ec)
	go func() {
		err := runFwdr(filepath.Join(runfiles, *fwdr), fwdrPort, svrPort)
		if err != nil {
			ec <- err
		}
	}()

	var conn net.Conn
	// Try to connect to the forwarder and retry in case the server has not come up yet
	for i := 0; i < 4; i++ {
		conn, err = net.Dial("tcp", "localhost:"+fwdrPort)
		if err != nil {
			time.Sleep(time.Millisecond * 300)
			continue
		}
		break
	}
	if conn == nil {
		t.Fatalf("Failed to connect to fwdr")
	}
	defer conn.Close()

	sent := fnv.New64a()
	recv := fnv.New64a()

	bts := make([]byte, 1024*16)
	for i := 0; i < len(bts); i++ {
		bts[i] = byte(i % 255)
	}

	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		for i := 0; i < 1024; i++ {
			rand.Shuffle(len(bts), func(i, j int) {
				bts[i], bts[j] = bts[j], bts[i]
			})
			sent.Write(bts)

			if _, err := conn.Write(bts); err != nil {
				ec <- err
			}
		}
		if _, err := conn.Write([]byte{255}); err != nil {
			ec <- err
		}
		wg.Done()
	}()

	go func() {
		if _, err := io.Copy(recv, conn); err != nil {
			ec <- err
		}
		wg.Done()
	}()

	done := make(chan struct{})
	go func() {
		defer close(done)
		wg.Wait()
	}()

	select {
	case <-done:
		if recv.Sum64() != sent.Sum64() {
			t.Errorf("Send bytes != Received bytes.")
		}
	case err := <-ec:
		t.Errorf("Got error: %v", err)
	case <-time.After(time.Second * 5):
		t.Errorf("timed out")
	}
}
