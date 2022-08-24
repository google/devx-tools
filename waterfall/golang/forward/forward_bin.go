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
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"github.com/google/waterfall/golang/forward"
	"github.com/google/waterfall/golang/mux"
	"github.com/google/waterfall/golang/net/qemu"
	"github.com/google/waterfall/golang/utils"
	"github.com/mdlayher/vsock"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sys/unix"
)

const (
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

type connBuilder interface {
	Accept() (net.Conn, error)
	Close() error
}

type vsockBuilder struct {
	cid  uint32
	port uint32
}

func (v *vsockBuilder) Accept() (net.Conn, error) {
	return vsock.Dial(v.cid, v.port, nil)
}

func (v *vsockBuilder) Close() error {
	return nil
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

func makeQemuBuilder(pa *utils.ParsedAddr) (connBuilder, error) {
	qb, err := qemu.MakeConnBuilder(pa.Addr, pa.SocketName)

	// The emulator can die at any point and we need to die as well.
	// When the emulator dies its working directory is removed.
	// Poll the filesystem and terminate the process if we can't
	// find the emulator dir.
	go func() {
		for {
			time.Sleep(sleepTime)
			if _, err := os.Stat(pa.Addr); err != nil {
				log.Fatal(err)
			}
		}
	}()
	return qb, err
}

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	log.Println("Starting forwarding server ...")

	if *listenAddr == "" || *connectAddr == "" {
		log.Fatalf("Need to specify -listen_addr and -connect_addr.")
	}

	log.Printf("Listening on address %s and connecting to %s\n", *listenAddr, *connectAddr)

	lisAddrs := []*utils.ParsedAddr{}
	for _, addr := range strings.Split(*listenAddr, ",") {
		lpa, err := utils.ParseAddr(addr)
		if err != nil {
			log.Fatal(err)
		}
		lisAddrs = append(lisAddrs, lpa)
	}

	cpa, err := utils.ParseAddr(*connectAddr)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	var b connBuilder
	switch cpa.Kind {
	case utils.QemuHost:
		qb, err := makeQemuBuilder(cpa)
		if err != nil {
			log.Fatalf("Got error creating qemu conn %v.", err)
		}
		defer qb.Close()
		b = qb
	case utils.QemuGuest:
		qb, err := qemu.MakePipe(cpa.SocketName)
		if err != nil {
			log.Fatalf("Got error creating qemu conn %v.", err)
		}
		defer qb.Close()
		b = qemu.MakePipeConnBuilder(qb)
	case utils.Unix:
		fallthrough
	case utils.TCP:
		b = &dialBuilder{netType: cpa.Kind, addr: cpa.Addr}
	case utils.VsockHost:
		cid, err := strconv.Atoi(cpa.Addr)
		if err != nil {
			log.Fatalf("Got error parsing vsock addr %v.", cpa)
		}

		port, err := strconv.Atoi(cpa.SocketName)
		if err != nil {
			log.Fatalf("Got error parsing vsock addr %v.", cpa)
		}
		b = &vsockBuilder{cid: uint32(cid), port: uint32(port)}
	case utils.Mux:
		mc, err := net.Dial(cpa.MuxAddr.Kind, cpa.MuxAddr.Addr)
		if err != nil {
			log.Fatal(err)
		}
		b, err = mux.NewConnBuilder(ctx, mc)
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
	cy.Close()

	listeners := []connBuilder{}
	for _, addr := range lisAddrs {
		switch addr.Kind {
		case utils.QemuHost:
			l, err := makeQemuBuilder(addr)
			if err != nil {
				log.Fatalf("Got error creating qemu conn %v.", err)
			}
			defer l.Close()
			listeners = append(listeners, l)
		case utils.Unix:
			fallthrough
		case utils.TCP:
			l, err := net.Listen(addr.Kind, addr.Addr)
			if err != nil {
				log.Fatalf("Failed to listen on address: %v.", err)
			}
			defer l.Close()
			listeners = append(listeners, l)
		}
	}

	go func() {
		sigc := make(chan os.Signal, 1)
		signal.Notify(sigc,
			unix.SIGTERM,
		)
		<-sigc
		log.Println("Received SIGTERM, closing connections...")
		for _, lis := range listeners {
			lis.Close()
		}
		b.Close()
	}()

	eg, _ := errgroup.WithContext(ctx)
	for i, lis := range listeners {
		func(ll connBuilder, pAddr *utils.ParsedAddr) {
			eg.Go(func() error {
				for {
					cx, err := ll.Accept()
					if err != nil {
						return err
					}
					cy, err := b.Accept()
					if err != nil {
						return err
					}
					log.Printf("Forwarding conns for addr %v ...\n", pAddr)
					go forward.Forward(cx.(forward.HalfReadWriteCloser), cy.(forward.HalfReadWriteCloser))
				}
				return nil
			})
		}(lis, lisAddrs[i])

	}

	if err := eg.Wait(); err != nil {
		log.Printf("Forwarding error: %v", err)
	}
}
