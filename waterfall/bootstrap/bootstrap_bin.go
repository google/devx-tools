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

// bootstrap_bin bootstraps the waterfall process through adb
package main

import (
	"context"
	"flag"
	"log"
	"path/filepath"
	"strings"

	"github.com/waterfall/adb"
	"github.com/waterfall/bootstrap"
)

var (
	// information requiered to bootstrap through adb.
	socketDir         = flag.String("socket_dir", "", "The sockets directory")
	qemuSocket        = flag.String("qemu_socket", "sockets/h2o", "The qemu pipe socket name.")
	forwarderBin      = flag.String("forwarder_bin", "", "The path to the forwarder binary.")
	waterfallBin      = flag.String("waterfall_bin", "", "Comma separated list arch:path server binaries.")
	adbBin            = flag.String("adb_bin", "", "Path to the adb binary to use.")
	adbDeviceName     = flag.String("adb_device_name", "", "The adb device name.")
	adbServerPort     = flag.String("adb_server_port", "5037", "The adb port of the adb server.")
	connectionTimeout = flag.Int("connection_timeout", 1000, "Connection timeout in ms.")
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
		sts = filepath.Dir(sts)
	}

	_, err := bootstrap.Bootstrap(context.Background(), adbConn, svrs, *forwarderBin, sts, *qemuSocket, *connectionTimeout)
	if err != nil {
		log.Fatalf("failed to bootstrap h2o: %v", err)
	}
}
