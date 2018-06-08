package main

import (
	"context"
	"flag"
	"log"

	"github.com/waterfall/net/qemu"
	waterfall_grpc "github.com/waterfall/proto/waterfall_go_grpc"
	"github.com/waterfall/server"
	"google.golang.org/grpc"
)

var mode = flag.String("mode", "qemu", "where to start the listener")

func main() {
	flag.Parse()

	log.Println("Starting waterfall server ...")
	if *mode != "qemu" {
		// for now we just support qemu connections
		log.Fatalf("Unsupported mode")
	}

	// listener backed by a qemu pipe
	lis, err := qemu.MakePipe()
	if err != nil {
		log.Fatalf("failed to listen %v", err)
	}

	grpcServer := grpc.NewServer()
	waterfall_grpc.RegisterWaterfallServer(
		grpcServer, server.NewWaterfallServer(context.Background()))

	log.Println("Serving ...")
	grpcServer.Serve(lis)
}
