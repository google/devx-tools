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
	"github.com/google/waterfall/golang/server"
	"github.com/google/waterfall/golang/testutils"
	waterfall_grpc_pb "github.com/google/waterfall/proto/waterfall_go_grpc"
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
	waterfall_grpc_pb.RegisterWaterfallServer(grpcServer, server.New())
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
	waterfall_grpc_pb.RegisterPortForwarderServer(grpcServer, NewServer(waterfall_grpc_pb.NewWaterfallClient(conn)))
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
func setup(t *testing.T) (waterfall_grpc_pb.PortForwarderClient, string) {
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
	return waterfall_grpc_pb.NewPortForwarderClient(conn), echoAddr
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

	s := &waterfall_grpc_pb.ForwardSession{Src: src, Dst: dst}
	ctx := context.Background()
	if _, err = c.ForwardPort(ctx, &waterfall_grpc_pb.PortForwardRequest{Session: s}); err != nil {
		t.Error(err)
		return
	}
	defer c.Stop(ctx, &waterfall_grpc_pb.PortForwardRequest{Session: s})

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
	s := &waterfall_grpc_pb.ForwardSession{Src: src, Dst: "tcp:foo"}
	ctx := context.Background()
	if _, err = c.ForwardPort(ctx, &waterfall_grpc_pb.PortForwardRequest{Session: s}); err != nil {
		t.Error(err)
		return
	}
	defer c.Stop(ctx, &waterfall_grpc_pb.PortForwardRequest{Session: s})

	// Rebind to a valid port and test we can talk to the server
	s.Dst = dst
	if _, err = c.ForwardPort(ctx, &waterfall_grpc_pb.PortForwardRequest{Session: s, Rebind: true}); err != nil {
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
	s := &waterfall_grpc_pb.ForwardSession{Src: src, Dst: dst}
	if _, err = c.ForwardPort(ctx, &waterfall_grpc_pb.PortForwardRequest{Session: s}); err != nil {
		t.Error(err)
		return
	}
	defer c.Stop(ctx, &waterfall_grpc_pb.PortForwardRequest{Session: s})

	s.Dst = "tcp:foo"
	if _, err = c.ForwardPort(ctx, &waterfall_grpc_pb.PortForwardRequest{Session: s, Rebind: false}); err == nil {
		t.Errorf("Was expecting error trying to rebind, but was successful.")
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

	s := &waterfall_grpc_pb.ForwardSession{Src: src, Dst: dst}
	ctx := context.Background()
	if _, err = c.ForwardPort(ctx, &waterfall_grpc_pb.PortForwardRequest{Session: s}); err != nil {
		t.Error(err)
		return
	}

	conn, err := net.Dial("tcp", srcAddr)
	if err != nil {
		t.Error(err)
		return
	}
	conn.Close()

	if _, err := c.Stop(ctx, &waterfall_grpc_pb.PortForwardRequest{Session: s}); err != nil {
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

	s := &waterfall_grpc_pb.ForwardSession{Src: src, Dst: dst}
	ctx := context.Background()
	if _, err = c.ForwardPort(ctx, &waterfall_grpc_pb.PortForwardRequest{Session: s}); err != nil {
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

	s = &waterfall_grpc_pb.ForwardSession{Src: src, Dst: dst}
	if _, err = c.ForwardPort(ctx, &waterfall_grpc_pb.PortForwardRequest{Session: s}); err != nil {
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

func TestList(t *testing.T) {
	c, _ := setup(t)
	f, err := testutils.PickUnusedPort()
	if err != nil {
		t.Fatal(err)
	}

	sessions := map[string]string{}

	srcAddrA := fmt.Sprintf("localhost:%d", f)
	src := fmt.Sprintf("tcp:%s", srcAddrA)
	dst := "tcp:foo"

	sessions[src] = dst

	s := &waterfall_grpc_pb.ForwardSession{Src: src, Dst: dst}
	ctx := context.Background()
	if _, err = c.ForwardPort(ctx, &waterfall_grpc_pb.PortForwardRequest{Session: s}); err != nil {
		t.Error(err)
		return
	}

	f, err = testutils.PickUnusedPort()
	if err != nil {
		t.Fatal(err)
	}
	srcAddrB := fmt.Sprintf("localhost:%d", f)
	src = fmt.Sprintf("tcp:%s", srcAddrB)

	sessions[src] = dst

	s = &waterfall_grpc_pb.ForwardSession{Src: src, Dst: dst}
	if _, err = c.ForwardPort(ctx, &waterfall_grpc_pb.PortForwardRequest{Session: s}); err != nil {
		t.Error(err)
		return
	}

	ss, err := c.List(ctx, &empty_pb.Empty{})
	if err != nil {
		t.Error(err)
		return
	}

	if len(ss.Sessions) != 2 {
		t.Errorf("expecting 2 forwarding sessions got %d", len(ss.Sessions))
		return
	}

	for _, fs := range ss.Sessions {
		if dst, ok := sessions[fs.Src]; !ok || fs.Dst != dst {
			t.Errorf("got unexpected forwarding session: %v", fs)
			return
		}
	}

}
