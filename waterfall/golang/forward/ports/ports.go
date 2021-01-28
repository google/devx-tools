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

// Package ports implements a grpc service to forward ports through waterfall.
// Although it's possible to forward a port directly in the waterfall client,
// the calling process would need to stay up for the duration of the forwarding session.
// This is not possible for command line tools that need to return immediately, so this
// is intended as a host daemon to keep persisten forwarding state.
package ports

import (
	"context"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"

	empty_pb "github.com/golang/protobuf/ptypes/empty"
	"github.com/google/waterfall/golang/forward"
	waterfall_grpc_pb "github.com/google/waterfall/proto/waterfall_go_grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type forwardSession struct {
	src    string
	dst    string
	lis    net.Listener
	cancel context.CancelFunc
}

// PortForwarder manages port forwarding sessions.
type PortForwarder struct {
	client               waterfall_grpc_pb.WaterfallClient
	sessions             map[string]*forwardSession
	sessionsMutex        *sync.Mutex
	reverseSessions      map[string]*forwardSession
	reverseSessionsMutex *sync.Mutex
}

// NewServer returns a new PortForwarder server.
func NewServer(client waterfall_grpc_pb.WaterfallClient) *PortForwarder {
	return &PortForwarder{
		client:               client,
		sessions:             make(map[string]*forwardSession),
		sessionsMutex:        &sync.Mutex{},
		reverseSessions:      make(map[string]*forwardSession),
		reverseSessionsMutex: &sync.Mutex{},
	}
}

func parseAddr(addr string) (string, string, error) {
	pts := strings.SplitN(addr, ":", 2)
	if len(pts) != 2 {
		return "", "", fmt.Errorf("failed to parse address %s", addr)
	}
	return pts[0], pts[1], nil
}

func networkKind(kind string) (waterfall_grpc_pb.ForwardMessage_Kind, error) {
	switch kind {
	case "tcp":
		return waterfall_grpc_pb.ForwardMessage_TCP, nil
	case "udp":
		return waterfall_grpc_pb.ForwardMessage_UDP, nil
	case "unix":
		return waterfall_grpc_pb.ForwardMessage_UNIX, nil
	default:
		return waterfall_grpc_pb.ForwardMessage_UNSET, status.Error(codes.InvalidArgument, "unknown network kind")
	}
}

func makeForwarder(ctx context.Context, c waterfall_grpc_pb.WaterfallClient, addr string, from net.Conn) (*forward.StreamForwarder, error) {
	kind, addr, err := parseAddr(addr)
	if err != nil {
		return nil, err
	}

	rpc, err := c.Forward(ctx)
	if err != nil {
		return nil, err
	}

	ntwk, err := networkKind(kind)
	if err != nil {
		return nil, err
	}

	if err := rpc.Send(&waterfall_grpc_pb.ForwardMessage{Kind: ntwk, Addr: addr}); err != nil {
		return nil, err
	}

	return forward.NewStreamForwarder(rpc, from.(forward.HalfReadWriteCloser)), nil
}

// ForwardPort starts forwarding the desired port.
func (pf *PortForwarder) ForwardPort(ctx context.Context, req *waterfall_grpc_pb.PortForwardRequest) (*empty_pb.Empty, error) {
	pf.sessionsMutex.Lock()
	defer pf.sessionsMutex.Unlock()

	log.Printf("Forwarding %s -> %s ...\n", req.Session.Src, req.Session.Dst)

	kind, addr, err := parseAddr(req.Session.Src)
	if err != nil {
		return nil, err
	}

	// Parse dst even if we don't use it. No point in sending it the server if its malformed.
	if _, _, err := parseAddr(req.Session.Dst); err != nil {
		return nil, err
	}

	if s, ok := pf.sessions[req.Session.Src]; ok {
		if !req.Rebind {
			return nil, status.Errorf(codes.AlreadyExists, "no-rebind specified, can't forward address: %s", req.Session.Src)
		}

		delete(pf.sessions, req.Session.Src)

		if err := s.lis.Close(); err != nil {
			return nil, err
		}
	}

	lis, err := net.Listen(kind, addr)
	if err != nil {
		return nil, err
	}

	// The context we create in this case is scoped to the duration of the forwarding
	// session, which outlives this request, therefore we can't propagate the request
	// context and are forced to create a new one.
	fCtx, cancel := context.WithCancel(context.Background())

	pf.sessions[req.Session.Src] = &forwardSession{src: req.Session.Src, dst: req.Session.Dst, lis: lis, cancel: cancel}
	go func() {
		defer cancel()
		defer lis.Close()
		for {
			conn, err := lis.Accept()
			if err != nil {
				// Theres really not much else we can do
				log.Printf("Error accepting connection: %v\n", err)
				break
			}
			fwdr, err := makeForwarder(fCtx, pf.client, req.Session.Dst, conn)
			if err != nil {
				log.Printf("Error creating forwarder: %v\n", err)
				conn.Close()
				continue
			}
			go fwdr.Forward()
		}
	}()

	return &empty_pb.Empty{}, nil
}

func (pf *PortForwarder) stopReverseForwarding(ctx context.Context, session *forwardSession) error {
	defer session.cancel()

	kind, addr, err := parseAddr(session.src)
	if err != nil {
		return err
	}
	delete(pf.reverseSessions, session.src)

	ntwk, err := networkKind(kind)
	if err != nil {
		return err
	}

	_, err = pf.client.StopReverseForward(
		ctx,
		&waterfall_grpc_pb.ForwardMessage{
			Op:   waterfall_grpc_pb.ForwardMessage_CLOSE,
			Kind: ntwk,
			Addr: addr})
	return err
}

// ReverseForwardPort forwards a port from the remote connection to a local port.
// Forwarding works as following:
// 1) The client (this process) sends a request to start forwarding a given port (StartReverseForward rpc)
//    and keeps the connection open to listen for new connections coming from the server.
// 2) When a new connection message is received, this process sends a new rpc to the server to hand the server a stream
//    on which to forward the connection (ReverseForward rpc).
// 3) The ReverseForward rpc is piped to the local port the streamed should be forwarded.
func (pf *PortForwarder) ReverseForwardPort(ctx context.Context, req *waterfall_grpc_pb.PortForwardRequest) (*empty_pb.Empty, error) {
	pf.reverseSessionsMutex.Lock()
	defer pf.reverseSessionsMutex.Unlock()

	log.Printf("Reverse forwarding %s -> %s ...\n", req.Session.Src, req.Session.Dst)
	if rs, ok := pf.reverseSessions[req.Session.Src]; ok {
		if !req.Rebind {
			return nil, status.Errorf(codes.AlreadyExists, "no-rebind specified, can't forward address: %s", req.Session.Src)
		}
		if err := pf.stopReverseForwarding(ctx, rs); err != nil {
			return nil, err
		}
	}

	srcKind, srcAddr, err := parseAddr(req.Session.Src)
	if err != nil {
		return nil, err
	}

	dstKind, dstAddr, err := parseAddr(req.Session.Dst)
	if err != nil {
		return nil, err
	}

	srcNtwk, err := networkKind(srcKind)
	if err != nil {
		return nil, err
	}

	// The context we create in this case is scoped to the duration of the forwarding
	// session, which outlives this request, therefore we can't propagate the request
	// context and are forced to create a new one.
	fCtx, cancel := context.WithCancel(context.Background())

	// 1) Ask the server to listen for connections on src
	ncs, err := pf.client.StartReverseForward(
		fCtx,
		&waterfall_grpc_pb.ForwardMessage{
			Op:   waterfall_grpc_pb.ForwardMessage_OPEN,
			Kind: srcNtwk,
			Addr: srcAddr})
	if err != nil {
		log.Printf("Failed to start reverse forwarding session (%s -> %s): %v", req.Session.Src, req.Session.Dst, err)
		return nil, err
	}

	ss := &forwardSession{src: req.Session.Src, dst: req.Session.Dst, cancel: cancel}
	pf.reverseSessions[req.Session.Src] = ss

	// Listen for new connections on a different goroutine so we can return to the client
	go func() {
		defer pf.stopReverseForwarding(fCtx, ss)
		for {
			log.Print("Waiting for new connection to forward...")
			fwd, err := ncs.Recv()
			if err != nil {
				log.Printf("Waterfall server error when listening for reverse forwarding connections: %v", err)
				return
			}

			if fwd.Op != waterfall_grpc_pb.ForwardMessage_OPEN {
				// The only type of message the server can reply is with an OPEN message
				log.Printf("Requested OP %v but only open is supported ...\n", fwd.Op)
				return
			}

			// 2) Hand the server a stream to start forwarding the connection
			fs, err := pf.client.ReverseForward(fCtx)
			if err != nil {
				log.Printf("Failed to create new forwarding session: %v", err)
				return
			}

			ntwk := srcNtwk
			addr := srcAddr
			if fwd.GetKind() != waterfall_grpc_pb.ForwardMessage_UNSET && len(fwd.Addr) > 0 {
				ntwk = fwd.Kind
				addr = fwd.Addr
			}
			if err := fs.Send(&waterfall_grpc_pb.ForwardMessage{
				Op:   waterfall_grpc_pb.ForwardMessage_OPEN,
				Kind: ntwk,
				Addr: addr,
			}); err != nil {
				log.Printf("Failed to create new forwarding request: %v", err)
				return
			}

			conn, err := net.Dial(dstKind, dstAddr)
			if err != nil {
				// Ignore this error. The socket might not be open initially
				// but can be created after the forwarding session.
				log.Printf("Failed to connect %s:%s: %v", dstKind, dstAddr, err)
				continue
			}

			// 3) Forward the stream to the connection
			go forward.NewStreamForwarder(fs, conn.(forward.HalfReadWriteCloser)).Forward()
		}
	}()
	return &empty_pb.Empty{}, nil
}

// StopReverse stops a reverse forwarding session killing all inflight connections.
func (pf *PortForwarder) StopReverse(ctx context.Context, req *waterfall_grpc_pb.PortForwardRequest) (*empty_pb.Empty, error) {
	pf.reverseSessionsMutex.Lock()
	defer pf.reverseSessionsMutex.Unlock()

	s, ok := pf.reverseSessions[req.Session.Src]
	if !ok {
		return &empty_pb.Empty{}, nil
	}
	return &empty_pb.Empty{}, pf.stopReverseForwarding(ctx, s)
}

// Stop stops a forwarding session killing all inflight connections.
func (pf *PortForwarder) Stop(ctx context.Context, req *waterfall_grpc_pb.PortForwardRequest) (*empty_pb.Empty, error) {
	pf.sessionsMutex.Lock()
	defer pf.sessionsMutex.Unlock()

	log.Printf("Stopping forwarding %s ...\n", req.Session.Src)

	s, ok := pf.sessions[req.Session.Src]
	if !ok {
		return &empty_pb.Empty{}, nil
	}
	s.cancel()

	delete(pf.sessions, req.Session.Src)
	return &empty_pb.Empty{}, s.lis.Close()
}

// StopAll stops all forwarding sessions and kills all inflight connections.
func (pf *PortForwarder) StopAll(ctx context.Context, req *empty_pb.Empty) (*empty_pb.Empty, error) {
	pf.sessionsMutex.Lock()
	defer pf.sessionsMutex.Unlock()

	log.Printf("Stopping all ...\n")

	for _, s := range pf.sessions {
		s.cancel()
		s.lis.Close()
	}
	pf.sessions = make(map[string]*forwardSession)

	return &empty_pb.Empty{}, nil
}

// List returns a list of all the currently forwarded connections.
func (pf *PortForwarder) List(ctx context.Context, req *empty_pb.Empty) (*waterfall_grpc_pb.ForwardedSessions, error) {
	ss := []*waterfall_grpc_pb.ForwardSession{}
	for _, s := range pf.sessions {
		ss = append(ss, &waterfall_grpc_pb.ForwardSession{Src: s.src, Dst: s.dst})
	}
	return &waterfall_grpc_pb.ForwardedSessions{Sessions: ss}, nil
}
