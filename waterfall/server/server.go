// Package server implements waterfall service
package server

import (
	"context"

	waterfall_grpc "github.com/waterfall/proto/waterfall_go_grpc"
	"io"
)

type waterfallServer struct {
	ctx context.Context
}

// Echo exists soley for test purposes. Its an utility function to
// create integration tests between grpc and custom network transports
// like qemu_pipe and usb
func (s *waterfallServer) Echo(stream waterfall_grpc.Waterfall_EchoServer) error {
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
func NewWaterfallServer(ctx context.Context) *waterfallServer {
	return &waterfallServer{ctx: ctx}
}
