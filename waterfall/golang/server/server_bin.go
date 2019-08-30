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
	"os"
	"syscall"

	"github.com/google/waterfall/golang/constants"
	"github.com/google/waterfall/golang/mux"
	"github.com/google/waterfall/golang/net/qemu"
	"github.com/google/waterfall/golang/server"
	"github.com/google/waterfall/golang/utils"
	waterfall_grpc "github.com/google/waterfall/proto/waterfall_go_grpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	_ "google.golang.org/grpc/encoding/gzip"
)

var (
	addr = flag.String(
		"addr", "qemu-guest:sockets/h2o", "Where to start listening. <qemu|tcp|unix|mux>:addr."+
			" If qemu is specified, addr is the name of the pipe socket."+
			" If mux addr is a file descriptor which is used to create mulitplexed connections.")
	sessionID = flag.String(
		"session_id", "", "Only accept requests with this string in x-session-id header."+
			" If empty, allow all requests.")

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

	var lis net.Listener
	var err error

	pa, err := utils.ParseAddr(*addr)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Starting %s:%s\n", pa.Kind, pa.Addr)

	switch pa.Kind {
	case utils.QemuGuest:
		lis, err = qemu.MakePipe(pa.SocketName)
		if err != nil {
			log.Fatalf("failed to open qemu_pipe %v", err)
		}
	case utils.Unix:
		fallthrough
	case utils.TCP:
		lis, err = net.Listen(pa.Kind, pa.Addr)
		if err != nil {
			log.Fatalf("Failed to listen %v", err)
		}
	case utils.Mux:
		lis = mux.NewListener(os.NewFile(uintptr(pa.MuxAddr.FD), ""))
	default:
		log.Fatalf("Unsupported kind %s", pa.Kind)
	}
	defer lis.Close()

	options := []grpc.ServerOption{grpc.WriteBufferSize(constants.WriteBufferSize)}
	if *cert != "" || *privateKey != "" {
		if *cert == "" || *privateKey == "" {
			log.Fatal("Need to specify -cert and -private_key")
		}

		creds, err := makeCredentials(*cert, *privateKey)
		if err != nil {
			log.Fatalf("failed to create tls credentials")
		}
		options = append(options, grpc.Creds(creds))
	} else {
		log.Println("Warning! running unsecure server ...")
	}

	if *sessionID != "" {
		log.Println("Verifying session ID for all requests.")
		ai := server.NewAuthInterceptor(*sessionID)
		options = append(
			options,
			grpc.UnaryInterceptor(ai.UnaryServerInterceptor),
			grpc.StreamInterceptor(ai.StreamServerInterceptor))
	}

	grpcServer := grpc.NewServer(options...)
	waterfall_grpc.RegisterWaterfallServer(grpcServer, new(server.WaterfallServer))

	log.Println("Serving ...")
	grpcServer.Serve(lis)
}
