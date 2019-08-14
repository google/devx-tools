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

// forward_bin listens on the -listen_addr and forwards the connection to -connect_addr
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/google/waterfall/golang/forward"
	"github.com/google/waterfall/golang/mux"
	"github.com/google/waterfall/golang/net/qemu"
	"golang.org/x/sync/errgroup"
)

const (
	qemuHost  = "qemu"
	qemuGuest = "qemu-guest"
	unixConn  = "unix"
	tcpConn   = "tcp"
	muxConn   = "mux"

	sleepTime = time.Millisecond * 2500
)

var (
	listenAddr = flag.String(
		"listen_addr", "",
		"List of address(es) separated by comma to listen for connection on the host. <unix|tcp>:addr1[,<unix|tcp>:addr2]")

	// For qemu connections addr is the working dir of the emulator
	connectAddr = flag.String(
		"connect_addr", "",
		"Connect and forward to this address <qemu|tcp|mux>:addr. For qemu connections addr is qemu:emu_dir:socket"+
			"where emu_dir is the working directory of the emulator and socket is the socket file relative to emu_dir."+
			" For mux connections addr is the initial connection to dial in order to create the mux connection builder.")
)

func init() {
	flag.Parse()
}

type parsedAddr struct {
	kind       string
	addr       string
	socketName string

	// The address to use for multiplexed connections.
	muxAddr *parsedAddr
}

func parseAddr(addr string) (*parsedAddr, error) {
	pts := strings.SplitN(addr, ":", 2)
	if len(pts) < 2 {
		return nil, fmt.Errorf("failed to parse address %s", addr)
	}

	if pts[0] == muxConn {
		mAddr, err := parseAddr(pts[1])
		if err != nil {
			return nil, err
		}
		return &parsedAddr{kind: pts[0], muxAddr: mAddr}, nil
	}

	host := pts[1]
	socket := ""
	if pts[0] == qemuHost || pts[0] == qemuGuest {
		p := strings.SplitN(pts[1], ":", 2)
		if len(p) != 2 {
			return nil, fmt.Errorf("failed to parse address %s", addr)
		}
		host = p[0]
		socket = p[1]
	}
	return &parsedAddr{kind: pts[0], addr: host, socketName: socket}, nil
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

func makeQemuBuilder(pa *parsedAddr) (connBuilder, error) {
	qb, err := qemu.MakeConnBuilder(pa.addr, pa.socketName)

	// The emulator can die at any point and we need to die as well.
	// When the emulator dies its working directory is removed.
	// Poll the filesystem and terminate the process if we can't
	// find the emulator dir.
	go func() {
		for {
			time.Sleep(sleepTime)
			if _, err := os.Stat(pa.addr); err != nil {
				log.Fatal(err)
			}
		}
	}()
	return qb, err
}

func main() {
	log.Println("Starting forwarding server ...")

	if *listenAddr == "" || *connectAddr == "" {
		log.Fatalf("Need to specify -listen_addr and -connect_addr.")
	}

	log.Printf("Listening on address %s and connecting to %s\n", *listenAddr, *connectAddr)

	lisAddrs := []*parsedAddr{}
	for _, addr := range strings.Split(*listenAddr, ",") {
		lpa, err := parseAddr(addr)
		if err != nil {
			log.Fatal(err)
		}
		lisAddrs = append(lisAddrs, lpa)
	}

	cpa, err := parseAddr(*connectAddr)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	var b connBuilder
	switch cpa.kind {
	case qemuHost:
		qb, err := makeQemuBuilder(cpa)
		if err != nil {
			log.Fatalf("Got error creating qemu conn %v.", err)
		}
		defer qb.Close()
		b = qb
	case qemuGuest:
		qb, err := qemu.MakePipe(cpa.socketName)
		if err != nil {
			log.Fatalf("Got error creating qemu conn %v.", err)
		}
		defer qb.Close()
		b = qb
	case unixConn:
		fallthrough
	case tcpConn:
		b = &dialBuilder{netType: cpa.kind, addr: cpa.addr}
	case muxConn:
		mc, err := net.Dial(cpa.muxAddr.kind, cpa.muxAddr.addr)
		if err != nil {
			log.Fatal(err)
		}
		b, err = mux.NewConnBuilder(ctx, mc)
		if err != nil {
			log.Fatal(err)
		}
	default:
		log.Fatalf("Unsupported network type: %s", cpa.kind)
	}

	// Block until the guest server process is ready.
	cy, err := b.Accept()
	if err != nil {
		log.Fatalf("Got error getting next conn: %v\n", err)
	}

	listeners := []connBuilder{}
	for _, addr := range lisAddrs {
		switch addr.kind {
		case qemuHost:
			l, err := makeQemuBuilder(addr)
			if err != nil {
				log.Fatalf("Got error creating qemu conn %v.", err)
			}
			defer l.Close()
			listeners = append(listeners, l)
		case unixConn:
			fallthrough
		case tcpConn:
			l, err := net.Listen(addr.kind, addr.addr)
			if err != nil {
				log.Fatalf("Failed to listen on address: %v.", err)
			}
			defer l.Close()
			listeners = append(listeners, l)
		}
	}

	eg, _ := errgroup.WithContext(ctx)
	for i, lis := range listeners {
		func(ll connBuilder, pAddr *parsedAddr) {
			eg.Go(func() error {
				for {
					cx, err := ll.Accept()
					if err != nil {
						return err
					}

					log.Printf("Forwarding conns for addr %v ...\n", pAddr)
					go forward.Forward(cx.(forward.HalfReadWriteCloser), cy.(forward.HalfReadWriteCloser))

					cy, err = b.Accept()
					if err != nil {
						return err
					}
				}
				return nil
			})
		}(lis, lisAddrs[i])

	}

	if err := eg.Wait(); err != nil {
		log.Fatalf("Forwarding error: %v", err)
	}
}
