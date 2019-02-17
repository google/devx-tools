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
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/google/waterfall/golang/forward"
	"github.com/google/waterfall/golang/net/qemu"
)

const (
	qemuHost  = "qemu"
	qemuGuest = "qemu-guest"
	unixConn  = "unix"
	tcpConn   = "tcp"

	sleepTime = time.Millisecond * 2500
)

var (
	listenAddr = flag.String("listen_addr", "", "Address to listen for connection on the host. <unix|tcp>:addr")

	// For qemu connections addr is the working dir of the emulator
	connectAddr = flag.String(
		"connect_addr", "",
		"Connect and forward to this address <qemu|tcp>:addr. For qemu connections addr is qemu:emu_dir:socket"+
			"where emu_dir is the working directory of the emulator and socket is the socket file relative to emu_dir.")
)

func init() {
	flag.Parse()
}

type parsedAddr struct {
	kind       string
	addr       string
	socketName string
}

func parseAddr(addr string) (*parsedAddr, error) {
	pts := strings.SplitN(addr, ":", 2)
	if len(pts) < 2 {
		return nil, fmt.Errorf("failed to parse address %s", addr)
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

	lpa, err := parseAddr(*listenAddr)
	if err != nil {
		log.Fatal(err)
	}

	cpa, err := parseAddr(*connectAddr)
	if err != nil {
		log.Fatal(err)
	}

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
	default:
		log.Fatalf("Unsupported network type: %s", cpa.kind)
	}

	// Block until the guest server process is ready.
	cy, err := b.Accept()
	if err != nil {
		log.Fatalf("Got error getting next conn: %v\n", err)
	}

	var lis connBuilder
	switch lpa.kind {
	case qemuHost:
		l, err := makeQemuBuilder(lpa)
		if err != nil {
			log.Fatalf("Got error creating qemu conn %v.", err)
		}
		defer l.Close()
		lis = l
	case unixConn:
		fallthrough
	case tcpConn:
		l, err := net.Listen(lpa.kind, lpa.addr)
		if err != nil {
			log.Fatalf("Failed to listen on address: %v.", err)
		}
		defer l.Close()
		lis = l
	}

	for {
		cx, err := lis.Accept()
		if err != nil {
			log.Fatalf("Got error accepting conn: %v\n", err)
		}

		log.Println("Forwarding conns ...")
		go forward.Forward(cx.(forward.HalfReadWriteCloser), cy.(forward.HalfReadWriteCloser))

		cy, err = b.Accept()
		if err != nil {
			log.Fatalf("Got error getting next conn: %v\n", err)
		}
	}
}
