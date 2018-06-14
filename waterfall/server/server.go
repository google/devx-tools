// Package server implements waterfall service
package server

import (
	"context"
	"io"

	waterfall_grpc "github.com/waterfall/proto/waterfall_go_grpc"
)

// WaterfallServer implements the waterfall gRPC service
type WaterfallServer struct {
	ctx context.Context
}

// Echo exists solely for test purposes. It's a utility function to
// create integration tests between grpc and custom network transports
// like qemu_pipe and usb
func (s *WaterfallServer) Echo(stream waterfall_grpc.Waterfall_EchoServer) error {
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if err := stream.Send(msg); err != nil {
			return err
		}
	}
}

// NewWaterfallServer returns a gRPC server that implements the waterfall service
func NewWaterfallServer(ctx context.Context) *WaterfallServer {
	return &WaterfallServer{ctx: ctx}
}
