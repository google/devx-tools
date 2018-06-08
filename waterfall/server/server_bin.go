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
	default:
		log.Fatalf("Unsupported mode %s", *mode)
	}

	grpcServer := grpc.NewServer()
	waterfall_grpc.RegisterWaterfallServer(
		grpcServer, server.NewWaterfallServer(context.Background()))

	log.Println("Serving ...")
	grpcServer.Serve(lis)
}
