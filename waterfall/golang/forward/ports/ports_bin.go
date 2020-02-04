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

// ports_bin starts a gRPC server that manages port forwarding sessions in the host.
package main

import (
	"flag"
	"log"
	"net"
	"strings"

	"github.com/google/waterfall/golang/forward/ports"
	waterfall_grpc_pb "github.com/google/waterfall/proto/waterfall_go_grpc"
	"google.golang.org/grpc"
)

var (
	addr          = flag.String("addr", "", "Address to listen for port forwarding requests. <unix|tcp>:addr")
	waterfallAddr = flag.String("waterfall_addr", "", "Address of the waterfall server. <unix|tcp>:addr")
)

func init() {
	flag.Parse()
}

func main() {
	log.Println("Starting port forwarding server ...")

	if *addr == "" || *waterfallAddr == "" {
		log.Fatalf("Need to specify -addr and -waterfall_addr.")
	}

	pts := strings.SplitN(*addr, ":", 2)
	if len(pts) != 2 {
		log.Fatalf("failed to parse address %s", *addr)
	}

	lis, err := net.Listen(pts[0], pts[1])
	if err != nil {
		log.Fatalf("Failed to listen %v", err)
	}

	conn, err := grpc.Dial(*waterfallAddr, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		log.Fatalf("Failed to establish connection to waterfall server: %v", err)
	}
	defer conn.Close()

	grpcServer := grpc.NewServer()
	waterfall_grpc_pb.RegisterPortForwarderServer(grpcServer, ports.NewServer(waterfall_grpc_pb.NewWaterfallClient(conn)))

	log.Println("Forwarding ports ...")
	grpcServer.Serve(lis)
}
