// Copyright 2019 Google LLC
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
	"github.com/waterfall/forward"
	waterfall_grpc "github.com/waterfall/proto/waterfall_go_grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type forwardSession struct {
	src    string
	lis    net.Listener
	cancel context.CancelFunc
}

// PortForwarder manages port forwarding sessions.
type PortForwarder struct {
	client        waterfall_grpc.WaterfallClient
	sessions      map[string]*forwardSession
	sessionsMutex *sync.Mutex
}

// NewServer returns a new PortForwarder server.
func NewServer(client waterfall_grpc.WaterfallClient) *PortForwarder {
	return &PortForwarder{
		client:        client,
		sessions:      make(map[string]*forwardSession),
		sessionsMutex: &sync.Mutex{},
	}
}

func parseAddr(addr string) (string, string, error) {
	pts := strings.SplitN(addr, ":", 2)
	if len(pts) != 2 {
		return "", "", fmt.Errorf("failed to parse address %s", addr)
	}
	return pts[0], pts[1], nil
}

func makeForwarder(ctx context.Context, c waterfall_grpc.WaterfallClient, addr string, from net.Conn) (*forward.StreamForwarder, error) {
	kind, addr, err := parseAddr(addr)
	if err != nil {
		return nil, err
	}

	rpc, err := c.Forward(ctx)
	if err != nil {
		return nil, err
	}

	var ntwk waterfall_grpc.ForwardMessage_Kind
	switch kind {
	case "tcp":
		ntwk = waterfall_grpc.ForwardMessage_TCP
	case "udp":
		ntwk = waterfall_grpc.ForwardMessage_UDP
	case "unix":
		ntwk = waterfall_grpc.ForwardMessage_UNIX
	}

	if err := rpc.Send(&waterfall_grpc.ForwardMessage{Kind: ntwk, Addr: addr}); err != nil {
		return nil, err
	}

	return forward.NewStreamForwarder(rpc, from.(forward.HalfReadWriteCloser)), nil
}

// ForwardPort starts forwarding the desired port.
func (pf *PortForwarder) ForwardPort(ctx context.Context, req *waterfall_grpc.PortForwardRequest) (*empty_pb.Empty, error) {
	pf.sessionsMutex.Lock()
	defer pf.sessionsMutex.Unlock()

	log.Printf("Forwarding %s -> %s ...\n", req.Src, req.Dst)

	kind, addr, err := parseAddr(req.Src)
	if err != nil {
		return nil, err
	}

	// Parse dst even if we don't use it. No point in sending it the server if its malformed.
	if _, _, err := parseAddr(req.Dst); err != nil {
		return nil, err
	}

	if s, ok := pf.sessions[req.Src]; ok {
		if !req.Rebind {
			return nil, status.Errorf(codes.AlreadyExists, "no-rebind specified, can't forward address: %s", req.Src)
		}

		delete(pf.sessions, req.Src)

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

	pf.sessions[req.Src] = &forwardSession{src: req.Src, lis: lis, cancel: cancel}
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
			fwdr, err := makeForwarder(fCtx, pf.client, req.Dst, conn)
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

// Stop stops a forwarding session killing all inflight connections.
func (pf *PortForwarder) Stop(ctx context.Context, req *waterfall_grpc.PortForwardRequest) (*empty_pb.Empty, error) {
	pf.sessionsMutex.Lock()
	defer pf.sessionsMutex.Unlock()

	log.Printf("Stopping forwarding %s ...\n", req.Src)

	s, ok := pf.sessions[req.Src]
	if !ok {
		return &empty_pb.Empty{}, nil
	}
	s.cancel()

	delete(pf.sessions, req.Src)
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
