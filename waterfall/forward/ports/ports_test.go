package ports

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	empty_pb "github.com/golang/protobuf/ptypes/empty"
	waterfall_grpc "github.com/waterfall/proto/waterfall_go_grpc"
	"github.com/waterfall/server"
	"github.com/waterfall/testutils"
	"google.golang.org/grpc"
)

func runEcho(t *testing.T, addr string) {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}

	for {
		conn, err := lis.Accept()
		if err != nil {
			t.Fatal(err)
		}
		go func() {
			defer conn.Close()
			io.Copy(conn, conn)
		}()
	}
}

func runWaterfallServer(t *testing.T, addr string) {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer lis.Close()

	grpcServer := grpc.NewServer()
	waterfall_grpc.RegisterWaterfallServer(grpcServer, new(server.WaterfallServer))
	grpcServer.Serve(lis)
}

func runPortForwarderServer(t *testing.T, addr, wtf string) {
	conn, err := grpc.Dial(wtf, grpc.WithInsecure())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer lis.Close()

	grpcServer := grpc.NewServer()
	waterfall_grpc.RegisterPortForwarderServer(grpcServer, NewServer(waterfall_grpc.NewWaterfallClient(conn)))
	grpcServer.Serve(lis)
}

func waitForServer(addr string, attempts int) error {
	for i := 0; i < attempts; i++ {
		if conn, err := net.Dial("tcp", addr); err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(time.Millisecond * 50)
	}
	return fmt.Errorf("failed to connect to forwarder server after %d attempts", attempts)
}

// setup spins up three servers to run the tests:
// 1) the waterfall server itself
// 2) the port forwarder server that talks to waterfall
// 3) an echo server to test forwarding is working as expected
func setup(t *testing.T) (waterfall_grpc.PortForwarderClient, string) {
	pe, err := testutils.PickUnusedPort()
	if err != nil {
		t.Fatal(err)
	}

	echoAddr := fmt.Sprintf("localhost:%d", pe)
	go runEcho(t, echoAddr)

	pw, err := testutils.PickUnusedPort()
	if err != nil {
		t.Fatal(err)
	}

	wtfAddr := fmt.Sprintf("localhost:%d", pw)
	go runWaterfallServer(t, wtfAddr)

	if err := waitForServer(wtfAddr, 5); err != nil {
		t.Fatal(err)
	}

	pf, err := testutils.PickUnusedPort()
	if err != nil {
		t.Fatal(err)
	}

	prtAddr := fmt.Sprintf("localhost:%d", pf)
	go runPortForwarderServer(t, prtAddr, wtfAddr)

	if err := waitForServer(prtAddr, 5); err != nil {
		t.Fatal(err)
	}

	conn, err := grpc.Dial(prtAddr, grpc.WithInsecure())
	if err != nil {
		t.Fatal(err)
	}
	return waterfall_grpc.NewPortForwarderClient(conn), echoAddr
}

func rwTest(conn net.Conn, msg []byte) error {
	expected := make([]byte, len(msg))
	copy(expected, msg)
	if _, err := conn.Write(msg); err != nil {
		return err
	}

	if _, err := io.ReadFull(conn, msg); err != nil {
		return err
	}

	if !bytes.Equal(msg, expected) {
		return fmt.Errorf("Unexpected response: %s", string(msg))
	}
	return nil
}

func TestForwardPort(t *testing.T) {
	c, echo := setup(t)

	f, err := testutils.PickUnusedPort()
	if err != nil {
		t.Fatal(err)
	}
	srcAddr := fmt.Sprintf("localhost:%d", f)
	src := fmt.Sprintf("tcp:%s", srcAddr)
	dst := fmt.Sprintf("tcp:%s", echo)

	ctx := context.Background()
	if _, err = c.ForwardPort(ctx, &waterfall_grpc.PortForwardRequest{Src: src, Dst: dst}); err != nil {
		t.Error(err)
		return
	}
	defer c.Stop(ctx, &waterfall_grpc.PortForwardRequest{Src: src})

	c1, err := net.Dial("tcp", srcAddr)
	if err != nil {
		t.Error(err)
		return
	}
	defer c1.Close()

	msg := []byte("hello")
	if err := rwTest(c1, msg); err != nil {
		t.Error(err)
		return
	}

	c2, err := net.Dial("tcp", srcAddr)
	if err != nil {
		t.Error(err)
		return
	}
	defer c2.Close()

	msg = []byte("hello as well")
	if err := rwTest(c2, msg); err != nil {
		t.Error(err)
	}
}

func TestForwardPortRebind(t *testing.T) {
	c, echo := setup(t)

	f, err := testutils.PickUnusedPort()
	if err != nil {
		t.Fatal(err)
	}
	srcAddr := fmt.Sprintf("localhost:%d", f)
	src := fmt.Sprintf("tcp:%s", srcAddr)
	dst := fmt.Sprintf("tcp:%s", echo)

	// Forward to a dummy port first
	ctx := context.Background()
	if _, err = c.ForwardPort(ctx, &waterfall_grpc.PortForwardRequest{Src: src, Dst: "tcp:foo"}); err != nil {
		t.Error(err)
		return
	}
	defer c.Stop(ctx, &waterfall_grpc.PortForwardRequest{Src: src})

	// Rebind to a valid port and test we can talk to the server
	if _, err = c.ForwardPort(ctx, &waterfall_grpc.PortForwardRequest{Src: src, Dst: dst, Rebind: true}); err != nil {
		t.Error(err)
		return
	}

	conn, err := net.Dial("tcp", srcAddr)
	if err != nil {
		t.Error(err)
		return
	}
	defer conn.Close()

	if err := rwTest(conn, []byte("hello")); err != nil {
		t.Error(err)
	}
}

func TestForwardPortNoRebind(t *testing.T) {
	c, echo := setup(t)

	f, err := testutils.PickUnusedPort()
	if err != nil {
		t.Fatal(err)
	}
	srcAddr := fmt.Sprintf("localhost:%d", f)
	src := fmt.Sprintf("tcp:%s", srcAddr)
	dst := fmt.Sprintf("tcp:%s", echo)

	ctx := context.Background()
	if _, err = c.ForwardPort(ctx, &waterfall_grpc.PortForwardRequest{Src: src, Dst: dst}); err != nil {
		t.Error(err)
		return
	}
	defer c.Stop(ctx, &waterfall_grpc.PortForwardRequest{Src: src})

	// Rebind to a valid port and test we can talk to the server
	if _, err = c.ForwardPort(ctx, &waterfall_grpc.PortForwardRequest{Src: src, Dst: "tcp:foo", Rebind: false}); err == nil {
		t.Errorf("Was expecting error trying to rebind, but was succesful.")
		return
	}

	// Make sure the original tunnel is still up
	conn, err := net.Dial("tcp", srcAddr)
	if err != nil {
		t.Error(err)
		return
	}
	defer conn.Close()

	if err := rwTest(conn, []byte("hello")); err != nil {
		t.Error(err)
	}
}

func TestStop(t *testing.T) {
	c, echo := setup(t)

	f, err := testutils.PickUnusedPort()
	if err != nil {
		t.Fatal(err)
	}
	srcAddr := fmt.Sprintf("localhost:%d", f)
	src := fmt.Sprintf("tcp:%s", srcAddr)
	dst := fmt.Sprintf("tcp:%s", echo)

	ctx := context.Background()
	if _, err = c.ForwardPort(ctx, &waterfall_grpc.PortForwardRequest{Src: src, Dst: dst}); err != nil {
		t.Error(err)
		return
	}

	conn, err := net.Dial("tcp", srcAddr)
	if err != nil {
		t.Error(err)
		return
	}
	conn.Close()

	if _, err := c.Stop(ctx, &waterfall_grpc.PortForwardRequest{Src: src}); err != nil {
		t.Error(err)
		return
	}
	if _, err = net.Dial("tcp", srcAddr); err == nil {
		t.Errorf("Succeded connecting to a closed forwarding session.")
	}

}

func TestStopAll(t *testing.T) {
	c, echo := setup(t)

	f, err := testutils.PickUnusedPort()
	if err != nil {
		t.Fatal(err)
	}

	srcAddrA := fmt.Sprintf("localhost:%d", f)
	src := fmt.Sprintf("tcp:%s", srcAddrA)
	dst := fmt.Sprintf("tcp:%s", echo)

	ctx := context.Background()
	if _, err = c.ForwardPort(ctx, &waterfall_grpc.PortForwardRequest{Src: src, Dst: dst}); err != nil {
		t.Error(err)
		return
	}

	conn, err := net.Dial("tcp", srcAddrA)
	if err != nil {
		t.Error(err)
		return
	}
	conn.Close()

	f, err = testutils.PickUnusedPort()
	if err != nil {
		t.Fatal(err)
	}

	srcAddrB := fmt.Sprintf("localhost:%d", f)
	src = fmt.Sprintf("tcp:%s", srcAddrB)
	dst = fmt.Sprintf("tcp:%s", echo)

	if _, err = c.ForwardPort(ctx, &waterfall_grpc.PortForwardRequest{Src: src, Dst: dst}); err != nil {
		t.Error(err)
		return
	}

	conn, err = net.Dial("tcp", srcAddrB)
	if err != nil {
		t.Error(err)
		return
	}
	conn.Close()

	if _, err := c.StopAll(ctx, &empty_pb.Empty{}); err != nil {
		t.Error(err)
		return
	}

	if _, err = net.Dial("tcp", srcAddrA); err == nil {
		t.Errorf("Succeded connecting to a closed forwarding session.")
	}

	if _, err = net.Dial("tcp", srcAddrB); err == nil {
		t.Errorf("Succeded connecting to a closed forwarding session.")
	}
}
