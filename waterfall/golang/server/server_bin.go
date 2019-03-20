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

// server runs a waterfall gRPC server
package main

import (
	"crypto/tls"
	"flag"
	"log"
	"net"
	"strings"
	"syscall"

	"github.com/google/waterfall/golang/constants"
	"github.com/google/waterfall/golang/net/qemu"
	"github.com/google/waterfall/golang/server"
	waterfall_grpc "github.com/google/waterfall/proto/waterfall_go_grpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	_ "google.golang.org/grpc/encoding/gzip"
)

var (
	addr = flag.String(
		"addr", "qemu:sockets/h2o", "Where to start listening. <qemu|tcp|unix>:addr."+
			" If qemu is specified, addr is the name of the pipe socket")

	cert       = flag.String("cert", "", "The path to the server certificate")
	privateKey = flag.String("private_key", "", "Path to the server private key")
)

func makeCredentials(cert, privateKey string) (credentials.TransportCredentials, error) {
	crt, err := tls.LoadX509KeyPair(cert, privateKey)
	if err != nil {
		return nil, err
	}

	return credentials.NewTLS(&tls.Config{Certificates: []tls.Certificate{crt}}), nil
}

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

	var grpcServer *grpc.Server
	if *cert != "" || *privateKey != "" {
		if *cert == "" || *privateKey == "" {
			log.Fatal("Need to specify -cert and -private_key")
		}

		creds, err := makeCredentials(*cert, *privateKey)
		if err != nil {
			log.Fatalf("failed to create tls credentials")
		}

		grpcServer = grpc.NewServer(
			grpc.WriteBufferSize(constants.WriteBufferSize),
			grpc.Creds(creds))
	} else {
		log.Println("Warning! running unsecure server ...")
		grpcServer = grpc.NewServer(grpc.WriteBufferSize(constants.WriteBufferSize))
	}

	waterfall_grpc.RegisterWaterfallServer(grpcServer, new(server.WaterfallServer))

	log.Println("Serving ...")
	grpcServer.Serve(lis)
}
