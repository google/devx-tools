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
package server

import (
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"strings"
	"syscall"

	"github.com/google/waterfall/golang/constants"
	"github.com/google/waterfall/golang/net/qemu"
	waterfall_grpc "github.com/google/waterfall/proto/waterfall_go_grpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	_ "google.golang.org/grpc/encoding/gzip"
)

func makeCredentials(cert, privateKey string) (credentials.TransportCredentials, error) {
	crt, err := tls.LoadX509KeyPair(cert, privateKey)
	if err != nil {
		return nil, err
	}

	return credentials.NewTLS(&tls.Config{Certificates: []tls.Certificate{crt}}), nil
}

type Provider struct {
	addr string
	cert string
	pKey string
}

func NewProvider(addr, cert, pKey string) *Provider {
	return &Provider{addr: addr, cert: cert, pKey: pKey}
}

func (p *Provider) Server(options ...grpc.ServerOption) (*grpc.Server, error) {
	// do not chown - owners and groups will not be valid.
	// adb will always create files with 0644 permission
	syscall.Umask(0)

	options = append(options, grpc.WriteBufferSize(constants.WriteBufferSize))
	if p.cert != "" || p.pKey != "" {
		if p.cert == "" || p.pKey == "" {
			return nil, fmt.Errorf("need to specify cert and private key")
		}
		creds, err := makeCredentials(p.cert, p.pKey)
		if err != nil {
			return nil, fmt.Errorf("failed to create tls credentials: %v", err)
		}
		options = append(options, grpc.Creds(creds))
	} else {
		log.Println("Warning! running insecure server ...")
	}

	grpcServer := grpc.NewServer(options...)
	waterfall_grpc.RegisterWaterfallServer(grpcServer, new(WaterfallServer))
	return grpcServer, nil
}

func (p *Provider) Listener() (net.Listener, error) {
	pts := strings.SplitN(p.addr, ":", 2)
	if len(pts) != 2 {
		return nil, fmt.Errorf("failed to parse address %s", p.addr)
	}

	kind := pts[0]
	loc := pts[1]
	var lis net.Listener
	var err error
	switch kind {
	case "qemu":
		lis, err = qemu.MakePipe(loc)
		if err != nil {
			err = fmt.Errorf("failed to open qemu_pipe %v", err)
		}
	case "unix":
		fallthrough
	case "tcp":
		lis, err = net.Listen(kind, loc)
		if err != nil {
			err = fmt.Errorf("failed to listen %v", err)
		}
	default:
		err = fmt.Errorf("unsupported kind %s", kind)
	}

	log.Printf("Listening on %s:%s\n", kind, loc)
	return lis, err
}
