// forward_bin listens on the -listen_addr and forwards the connection to -connect_addr
package main

import (
	"flag"
	"fmt"
	"github.com/waterfall/net/qemu"
	"io"
	"log"
	"net"
	"strings"
	"sync"
)

const (
	qemuConn = "qemu"
	unixConn = "unix"
	tcpConn  = "tcp"
)

var (
	// For qemu connections addr is the working dir of the emulator
	listenAddr  = flag.String("listen_addr", "", "Address to listen for connection on the host. <unix|tcp>:addr")
	connectAddr = flag.String("connect_addr", "", "Connect and forward to this address <qemu:tcp>:addr")
)

func init() {
	flag.Parse()
}

func parseAddr(addr string) (string, string, error) {
	pts := strings.SplitN(addr, ":", 2)
	if len(pts) < 2 {
		return "", "", fmt.Errorf("failed to parse address %s.", addr)
	}
	return pts[0], pts[1], nil
}

type connBuilder interface {
	Next() (net.Conn, error)
}

type dialBuilder struct {
	netType string
	addr    string
}

func (d *dialBuilder) Next() (net.Conn, error) {
	return net.Dial(d.netType, d.addr)
}

func asyncCopy(w io.Writer, r io.ReadCloser, wg *sync.WaitGroup, ch chan error) {
	go func() {
		_, err := io.Copy(w, r)
		if err != nil {
			ch <- err
			return
		}
		wg.Done()
	}()
}

func forward(x net.Conn, y net.Conn) {
	defer x.Close()
	defer y.Close()

	wg := &sync.WaitGroup{}

	// wait for the outgoing and incoming copy goroutines
	wg.Add(2)

	doneCh := make(chan struct{}, 1)

	// allow both the outgoing and incoming goroutine to send to the channel
	errCh := make(chan error, 2)
	asyncCopy(x, y, wg, errCh)
	asyncCopy(y, x, wg, errCh)

	go func() {
		wg.Wait()
		doneCh <- struct{}{}
	}()

	select {
	case <-doneCh:
	case <-errCh: // just ignore, there's not much we can do.
	}
}

func main() {
	log.Println("Starting forwarding server ...")

	if *listenAddr == "" || *connectAddr == "" {
		log.Fatalf("Need to specify -listen_addr and -connect_addr.")
	}

	lt, la, err := parseAddr(*listenAddr)
	if err != nil {
		log.Fatal(err)
	}

	ct, ca, err := parseAddr(*connectAddr)
	if err != nil {
		log.Fatal(err)
	}

	lis, err := net.Listen(lt, la)
	if err != nil {
		log.Fatalf("Failed to listen on address: %v.", err)
	}

	var b connBuilder
	switch ct {
	case qemuConn:
		qb, err := qemu.MakeConnBuilder(ca)
		if err != nil {
			log.Fatalf("Got error creating qemu conn %v.", err)
		}
		defer qb.Close()
		b = qb
	case unixConn:
		b = &dialBuilder{netType: ct, addr: ca}
	case tcpConn:
		b = &dialBuilder{netType: ct, addr: ca}
	default:
		log.Fatalf("Unsupported network type: %s", ct)
	}

	for {
		cx, err := lis.Accept()
		if err != nil {
			log.Fatalf("Got error accepting conn: %v\n", err)
		}

		cy, err := b.Next()
		if err != nil {
			log.Fatalf("Got error getting next conn: %v\n", err)
		}
		log.Println("Forwarding ...")
		go forward(cx, cy)
	}
}
