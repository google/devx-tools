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

// forward_usb_bin listens on the -listen_addr and forwards the connection to the usb serial at -connect_addr
package main

import (
	"context"
	"flag"
	"log"
	"net"

	"github.com/google/waterfall/golang/aoa"
	"github.com/google/waterfall/golang/forward"
	"github.com/google/waterfall/golang/mux"
	"github.com/google/waterfall/golang/utils"
)

var (
	listenAddr = flag.String("listen_addr", "", "Address to listen for connection on the host. <unix|tcp>:addr")
	connectAddr = flag.String("connect_addr", "", "USB serial port to connect and multiplex. mux:usb:usb_serial.")
)

func init() {
	flag.Parse()
}

type connBuilder interface {
	Accept() (net.Conn, error)
	Close() error
}

type dialBuilder struct {
	netType string
	addr    string
}

func (d *dialBuilder) Accept() (net.Conn, error) {
	return net.Dial(d.netType, d.addr)
}

func (d *dialBuilder) Close() error {
	return nil
}

func main() {
	log.Println("Starting forwarding server ...")

	if *listenAddr == "" || *connectAddr == "" {
		log.Fatalf("Need to specify -listen_addr and -connect_addr.")
	}

	log.Printf("Listening on address %s and connecting to %s\n", *listenAddr, *connectAddr)

	cpa, err := utils.ParseAddr(*connectAddr)
	if err != nil {
		log.Fatal(err)
	}

	lpa, err := utils.ParseAddr(*listenAddr)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	var b connBuilder
	switch cpa.Kind {
	case utils.Unix:
		fallthrough
	case utils.TCP:
		b = &dialBuilder{netType: cpa.Kind, addr: cpa.Addr}
	case utils.Mux:
		mx, err := aoa.Connect(cpa.MuxAddr.Addr)
		if err != nil {
			log.Fatal(err)
		}
		b, err = mux.NewConnBuilder(ctx, mx)
		if err != nil {
			log.Fatal(err)
		}
	default:
		log.Fatalf("Unsupported network type: %s", cpa.Kind)
	}

	// Block until the guest server process is ready.
	cy, err := b.Accept()
	if err != nil {
		log.Fatalf("Got error getting next conn: %v\n", err)
	}

	var lis connBuilder
	switch lpa.Kind {
	case utils.Unix:
		fallthrough
	case utils.TCP:
		lis, err = net.Listen(lpa.Kind, lpa.Addr)
		if err != nil {
			log.Fatalf("Failed to listen on address: %v.", err)
		}
		defer lis.Close()
	default:
		log.Fatalf("Unsupported network type: %s", cpa.Kind)
	}

	for {
		cx, err := lis.Accept()
		if err != nil {
			log.Fatal(err)
		}

		log.Println("Forwarding conns ...")
		go forward.Forward(cx.(forward.HalfReadWriteCloser), cy.(forward.HalfReadWriteCloser))

		cy, err = b.Accept()
		if err != nil {
			log.Fatal(err)
		}
	}
}
