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

// server runs a waterfall gRPC server
package main

import (
	"flag"
	"log"
	"net"
	"strings"
	"syscall"

	"github.com/waterfall"
	"github.com/waterfall/net/qemu"
	waterfall_grpc "github.com/waterfall/proto/waterfall_go_grpc"
	"github.com/waterfall/server"
	"google.golang.org/grpc"
	_ "google.golang.org/grpc/encoding/gzip"
)

var addr = flag.String(
	"addr", "qemu:sockets/h2o", "Where to start listening. <qemu|tcp|unix>:addr."+
		" If qemu is specified, addr is the name of the pipe socket")

func main() {
	flag.Parse()

	// do not chown - owners and groups will not be valid.
	// adb will always create files with 0644 permission
	syscall.Umask(0)

	log.Println("Starting waterfall server ...")

	if *addr == "" {
		log.Fatalf("Need to specify -addr.")
	}

	pts := strings.SplitN(*addr, ":", 2)
	if len(pts) != 2 {
		log.Fatalf("Failed to parse addres %s", *addr)
	}

	kind := pts[0]
	loc := pts[1]

	log.Printf("Starting %s:%s\n", kind, loc)

	var lis net.Listener
	var err error
	switch kind {
	case "qemu":
		lis, err = qemu.MakePipe(loc)
		if err != nil {
			log.Fatalf("failed to open qemu_pipe %v", err)
		}
	case "unix":
		fallthrough
	case "tcp":
		lis, err = net.Listen(kind, loc)
		if err != nil {
			log.Fatalf("Failed to listen %v", err)
		}

	default:
		log.Fatalf("Unsupported kind %s", kind)
	}

	grpcServer := grpc.NewServer(grpc.WriteBufferSize(waterfall.WriteBufferSize))
	waterfall_grpc.RegisterWaterfallServer(grpcServer, new(server.WaterfallServer))

	log.Println("Serving ...")
	grpcServer.Serve(lis)
}
