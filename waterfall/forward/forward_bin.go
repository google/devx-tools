// forward_bin listens on the -listen_addr and forwards the connection to -connect_addr
package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strings"

	"github.com/waterfall/adb"
	"github.com/waterfall/forward"
	"github.com/waterfall/net/qemu"
)

const (
	qemuConn = "qemu"
	unixConn = "unix"
	tcpConn  = "tcp"
)

var (
	bootstrap = flag.Bool("bootstrap", false, "If bootstrap is true, h2o bootsraps itself through adb.")

	// information requiered to bootstrap through adb.
	qemuSocket    = flag.String("qemu_socket", "sockets/h2o", "The qemu pipe socket name.")
	waterfallBin  = flag.String("waterfall_bin", "", "Comma separated list arch:path server binaries.")
	adbBin        = flag.String("adb_bin", "", "Path to the adb binary to use.")
	adbDeviceName = flag.String("adb_device_name", "", "The adb device name.")
	adbServerPort = flag.String("adb_server_port", "5037", "The adb port of the adb server.")
	statusFile    = flag.String("status_file", "", "Output infomation about the waterfall process to this file.")

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
	if pts[0] == qemuConn {
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
	Next() (net.Conn, error)
}

type dialBuilder struct {
	netType string
	addr    string
}

func (d *dialBuilder) Next() (net.Conn, error) {
	return net.Dial(d.netType, d.addr)
}

func main() {
	log.Println("Starting forwarding server ...")

	if *bootstrap {

		if *waterfallBin == "" || *qemuSocket == "" || *adbBin == "" || *adbDeviceName == "" || *adbServerPort == "" {
			log.Fatalf(
				"need to provide -waterfall_bin, -qemu_socket, -adb_bin, " +
					"-device_name, -adb_server_port, and -status_file when bootstrapping.")
		}

		svrs := strings.Split(*waterfallBin, ",")
		adbConn := &adb.Device{
			AdbPath:       *adbBin,
			DeviceName:    *adbDeviceName,
			AdbServerPort: *adbServerPort}

		// make sure the device is connected before anything else
		if err := adbConn.Connect(); err != nil {
			log.Fatalf("Error connecting to device %s: %v", adbConn.DeviceName, err)
		}

		_, err := forward.Bootstrap(adbConn, svrs, os.Args[0], *qemuSocket)
		if err != nil {
			log.Fatalf("failed to bootstrap h2o: %v", err)
		}
		return
	}

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

	lis, err := net.Listen(lpa.kind, lpa.addr)
	if err != nil {
		log.Fatalf("Failed to listen on address: %v.", err)
	}
	defer lis.Close()

	var b connBuilder
	switch cpa.kind {
	case qemuConn:
		qb, err := qemu.MakeConnBuilder(cpa.addr, cpa.socketName)
		if err != nil {
			log.Printf("Got error creating qemu conn %v.", err)
		}
		defer qb.Close()
		b = qb
	case unixConn:
		b = &dialBuilder{netType: cpa.kind, addr: cpa.addr}
	case tcpConn:
		b = &dialBuilder{netType: cpa.kind, addr: cpa.addr}
	default:
		log.Fatalf("Unsupported network type: %s", cpa.kind)
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
		go forward.Forward(cx.(forward.HalfReadWriteCloser), cy.(forward.HalfReadWriteCloser))
	}
}
