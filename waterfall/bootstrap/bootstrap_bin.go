// bootstrap_bin bootstraps the waterfall process through adb
package main

import (
	"flag"
	"log"
	"path/filepath"
	"strings"

	"github.com/waterfall/adb"
	"github.com/waterfall/bootstrap"
)

var (
	// information requiered to bootstrap through adb.
	socketDir     = flag.String("socket_dir", "", "The sockets directory")
	qemuSocket    = flag.String("qemu_socket", "sockets/h2o", "The qemu pipe socket name.")
	forwarderBin  = flag.String("forwarder_bin", "", "The path to the forwarder binary.")
	waterfallBin  = flag.String("waterfall_bin", "", "Comma separated list arch:path server binaries.")
	adbBin        = flag.String("adb_bin", "", "Path to the adb binary to use.")
	adbDeviceName = flag.String("adb_device_name", "", "The adb device name.")
	adbServerPort = flag.String("adb_server_port", "5037", "The adb port of the adb server.")
)

func init() {
	flag.Parse()
}

type parsedAddr struct {
	kind       string
	addr       string
	socketName string
}

func main() {
	log.Println("Bootstrping the waterfall process ...")

	if *forwarderBin == "" || *waterfallBin == "" || *qemuSocket == "" || *adbBin == "" || *adbDeviceName == "" || *adbServerPort == "" {
		log.Fatalf(
			"need to provide -forwarder_bin, -waterfall_bin, -qemu_socket, -adb_bin, " +
				"-adb_device_name and -adb_server_port when bootstrapping.")
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

	sts := *socketDir
	if sts == "" {
		var err error
		if sts, err = adbConn.QemuPipeDir(); err != nil {
			log.Fatalf("error getting pipe dir: %v", err)
		}
		sts = filetpath.Dir(sts)
	}

	_, err := bootstrap.Bootstrap(adbConn, svrs, *forwarderBin, sts, *qemuSocket)
	if err != nil {
		log.Fatalf("failed to bootstrap h2o: %v", err)
	}
}
