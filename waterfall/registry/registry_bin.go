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

// registry is a simple key-value store gRPC service
package main

import (
	"flag"
	"log"
	"net"
	"strings"

	waterfall_grpc "github.com/waterfall/proto/waterfall_go_grpc"
	"github.com/waterfall/registry"
	"google.golang.org/grpc"
)

var (
	addr     = flag.String("addr", "localhost:13331", "network address in which to listen")
	unixPipe = flag.String(
		"unix_pipe", "h2o_waterfall_registry", "Name of abstract unix pipe to listen on")
	mode    = flag.String("mode", "unix", "Type of connection <unix|TCP>")
	entries = flag.String("entries", "", "List of comma separated k=v entries")
)

func main() {
	flag.Parse()

	log.Println("Starting kv store in port ...")

	var lis net.Listener
	var err error
	switch *mode {
	case "unix":
		lis, err = net.Listen("unix", "@"+*unixPipe)
	case "tcp":
		lis, err = net.Listen("tcp", *addr)
	default:
		log.Fatalf("Unuspported mode %s", *mode)
	}

	if err != nil {
		log.Fatalf("failed to listen %v", err)
	}
	defer lis.Close()

	es := make(map[string]string)
	if *entries != "" {
		for _, e := range strings.Split(*entries, ",") {
			pts := strings.Split(e, "=")
			if len(pts) != 2 {
				log.Fatalf("Malformed entry %s", e)
			}
			es[pts[0]] = pts[1]
		}
	}

	grpcServer := grpc.NewServer()
	waterfall_grpc.RegisterRegistryServer(grpcServer, registry.NewServerWithEntries(es))

	log.Println("Serving ...")
	grpcServer.Serve(lis)
}
