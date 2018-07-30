// server runs a waterfall gRPC server
package main

import (
	"context"
	"flag"
	"log"
	"net"

	"github.com/waterfall/net/qemu"
	waterfall_grpc "github.com/waterfall/proto/waterfall_go_grpc"
	"github.com/waterfall/server"
	"google.golang.org/grpc"
)

var mode = flag.String("mode", "qemu", "where to start the listener")
var qemuSocket = flag.String("qemu_socket", "sockets/h2o", "The socket name in the host")
var addr = flag.String("addr", "localhost:8088", "where to listen for connections")

func main() {
	flag.Parse()

	log.Println("Starting waterfall server ...")

	var lis net.Listener
	var err error

	// for now we just support qemu
	switch *mode {
	case "qemu":
		lis, err = qemu.MakePipe()
		if err != nil {
			log.Fatalf("failed to open qemu_pipe %v", err)
		}
	case "unix":
		fallthrough
	case "tcp":
		lis, err = net.Listen(*mode, *addr)
		if err != nil {
			log.Fatalf("Failed to listen %v", err)
		}

	default:
		log.Fatalf("Unsupported mode %s", *mode)
	}

	grpcServer := grpc.NewServer()
	waterfall_grpc.RegisterWaterfallServer(
		grpcServer, server.NewWaterfallServer(context.Background()))

	log.Println("Serving ...")
	grpcServer.Serve(lis)
}
