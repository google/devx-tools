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
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/google/waterfall/golang/constants"
	"github.com/google/waterfall/golang/mux"
	"github.com/google/waterfall/golang/net/qemu"
	"github.com/google/waterfall/golang/server"
	"github.com/google/waterfall/golang/utils"
	waterfall_grpc_pb "github.com/google/waterfall/proto/waterfall_go_grpc"
	"github.com/mdlayher/vsock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	_ "google.golang.org/grpc/encoding/gzip"
)

var (
	addr = flag.String(
		"addr", "qemu-guest:sockets/h2o", "Where to start listening. <qemu|qemu2|tcp|unix|mux>:addr."+
			" If qemu is specified, addr is the name of the pipe socket."+
			" If mux addr is a file descriptor which is used to create mulitplexed connections.")
	sessionID = flag.String(
		"session_id", "", "Only accept requests with this string in x-session-id header."+
			" If empty, allow all requests.")

	cert       = flag.String("cert", "", "The path to the server certificate")
	privateKey = flag.String("private_key", "", "Path to the server private key")
	daemon     = flag.Bool("daemon", false, "If true the server runs in daemon mode")

)

func makeCredentials(cert, privateKey string) (credentials.TransportCredentials, error) {
	crt, err := tls.LoadX509KeyPair(cert, privateKey)
	if err != nil {
		return nil, err
	}

	return credentials.NewTLS(&tls.Config{Certificates: []tls.Certificate{crt}}), nil
}

func forkInDaemonMode(path string, paddr *utils.ParsedAddr, sessionID, cert, privateKey string) error {
	a := fmt.Sprintf("%s:%s", paddr.Kind, paddr.Addr)
	ef := []*os.File{}
	if paddr.Kind == utils.TCP {
		// allocate port before forking to avoid race conditions
		// this is specially useful for situations where port is unknown (i.e :0)
		// This way we can pick port here and hand it to the child.
		// This way child doesent need to talk to the parent.
		lis, err := net.Listen("tcp", paddr.Addr)
		if err != nil {
			return err
		}

		l, _ := lis.(*net.TCPListener)

		pts := strings.Split(l.Addr().String(), ":")
		fmt.Printf("Waterfall server port: [%s]\n", pts[len(pts)-1])
		f, err := l.File()
		if err != nil {
			return err
		}
		a = "fd:3"
		ef = append(ef, f)
	}

	cmd := exec.Command(
		path, "-addr", a, "-daemon=false", "-session_id", sessionID, "-cert", cert, "-private_key", privateKey)
	cmd.ExtraFiles = ef
	return cmd.Start()
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
	if *daemon {
		if err := forkInDaemonMode(os.Args[0], pa, *sessionID, *cert, *privateKey); err != nil {
			log.Fatalf("Failed to start waterfall in daemon mode: %v", err)
		}
		return
	}

	switch pa.Kind {
	case utils.QemuGuest:
		pip, err := qemu.MakePipe(pa.SocketName)
		if err != nil {
			log.Fatalf("failed to open qemu_pipe %v", err)
		}
		lis = qemu.MakePipeConnBuilder(pip)
	case utils.QemuCtrl:
		pip, err := qemu.MakePipe(pa.SocketName)
		if err != nil {
			log.Fatalf("failed to open qemu_pipe %v", err)
		}
		lis = qemu.MakeControlSocket(pip)
	case utils.VsockGuest:
		p, err := strconv.Atoi(pa.SocketName)
		if err != nil {
			log.Fatalf("Failed to parse vsock: %v", pa)
		}
		lis, err = vsock.Listen(uint32(p))
		if err != nil {
			log.Fatalf("failed to open vsock %v", err)
		}
	case utils.Unix:
		fallthrough
	case utils.TCP:
		lis, err = net.Listen(pa.Kind, pa.Addr)
		if err != nil {
			log.Fatalf("Failed to listen %v", err)
		}
	case utils.FD:
		lis, err = net.FileListener(os.NewFile(uintptr(pa.FD), ""))
		if err != nil {
			log.Fatalf("Failed to create fd listener %v", err)
		}
	case utils.Mux:
		lis = mux.NewListener(os.NewFile(uintptr(pa.MuxAddr.FD), ""))
	default:
		log.Fatalf("Unsupported kind %s", pa.Kind)
	}
	defer lis.Close()

	options := []grpc.ServerOption{
		grpc.WriteBufferSize(constants.WriteBufferSize),
		// Don't timeout connections (1 year timeout)
		grpc.ConnectionTimeout(8760 * time.Hour),
	}
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
	waterfall_grpc_pb.RegisterWaterfallServer(grpcServer, server.New())

	log.Println("Serving ...")
	grpcServer.Serve(lis)
}
