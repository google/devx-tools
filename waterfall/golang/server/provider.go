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

// WFServer provides a GRPC server registered to serve Waterfall API. Providing cert and pKey
// configures the server for TLS, options could be added to further configure the server.
func WFServer(cert string, pKey string, options ...grpc.ServerOption) (*grpc.Server, error) {
	options = append(options, grpc.WriteBufferSize(constants.WriteBufferSize))
	if cert != "" || pKey != "" {
		if cert == "" || pKey == "" {
			return nil, fmt.Errorf("need to specify cert and private key")
		}
		creds, err := makeCredentials(cert, pKey)
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

// WFListener provides a default waterfall listener where the GRPC server can listen for new
// requests.
func WFListener(addr string) (net.Listener, error) {
	pts := strings.SplitN(addr, ":", 2)
	if len(pts) != 2 {
		return nil, fmt.Errorf("failed to parse address %s", addr)
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
