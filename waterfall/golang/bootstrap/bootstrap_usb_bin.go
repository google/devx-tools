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

// bootstrap_usb_bin bootstraps the waterfall process through adb and starts the AoA USB transport.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"time"

	"github.com/google/waterfall/golang/adb"
	"google.golang.org/grpc"
)

var (
	// information required to bootstrap through adb.
	usbServiceAPK        = flag.String("usb_service_apk", "", "Path to the Waterfall USB service APK.")
	forwarderBin         = flag.String("forwarder_bin", "", "Path to the forwarder binary.")
	waterfallBin         = flag.String("waterfall_bin", "", "Path to the Waterfall daemon.")
	forkfdBin            = flag.String("forkfd_bin", "", "Path to the forkfd binary.")
	adbBin               = flag.String("adb_bin", "", "Path to the adb binary to use.")
	adbDeviceSerial      = flag.String("adb_device_serial", "", "The adb device serial number.")
	adbServerPort        = flag.String("adb_server_port", "5037", "The adb port of the adb server.")
	waterfallServicePort = flag.String("waterfall_service_port", "8080", "Port number where the Waterfall daemon will listen for connections from the USB service.")
	connectionTimeout    = flag.Int("connection_timeout", 1000, "Connection timeout in ms.")
)

const (
	waterfallDevicePath = "/data/local/tmp/waterfall"
	forkfdDevicePath    = "/data/local/tmp/forkfd"
	usbPackageName      = "com.google.waterfall.usb"
	sdkProp             = "ro.build.version.sdk"
)

func init() {
	flag.Parse()
}

func main() {

	if *forwarderBin == "" || *waterfallBin == "" || *forkfdBin == "" || *adbBin == "" || *adbDeviceSerial == "" || *adbServerPort == "" || *forkfdBin == "" || *usbServiceAPK == "" {
		log.Fatalf("Need to provide -forwarder_bin, -waterfall_bin, -forkfd_bin, -adb_bin and -adb_device_serial when bootstrapping.")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*1)
	defer cancel()

	devAddr := fmt.Sprintf("unix:@h2o_%s", *adbDeviceSerial)
	if _, err := grpc.DialContext(ctx, devAddr, grpc.WithBlock(), grpc.WithInsecure()); err == nil {
		log.Println("Nothing to do. Waterfall already running.")
		return
	}

	log.Println("Bootstrapping Waterfall USB ...")
	adbConn := &adb.Device{
		AdbPath:       *adbBin,
		DeviceName:    *adbDeviceSerial,
		AdbServerPort: *adbServerPort}

	// make sure the device is connected to ADB before anything else.
	if err := adbConn.Connect(); err != nil {
		log.Fatalf("Error connecting to device %s: %v", adbConn.DeviceName, err)
	}

	// TODO(mauriciogg): avoid pushing whenever possible.
	log.Println("Pushing binaries ...")
	if err := adbConn.Push(*waterfallBin, waterfallDevicePath); err != nil {
		log.Fatal(err)
	}

	if err := adbConn.Push(*forkfdBin, forkfdDevicePath); err != nil {
		log.Fatal(err)
	}

	log.Println("Installing USB service ...")
	if out, err := adbConn.Install(*usbServiceAPK); err != nil {
		log.Fatalf("Failed to install USB service: %v\n %s", err, out)
	}

	// Kill zombie processes
	adbConn.Shell([]string{"pkill", "forkfd"})
	adbConn.Shell([]string{"pkill", "waterfall"})
	adbConn.Shell([]string{"am", "force-stop", usbPackageName})

	// Kickstart AOA
	log.Println("Starting services ...")
	out, err := adbConn.Shell(
		[]string{
			forkfdDevicePath, "-fork", "-binary_to_launch", waterfallDevicePath, "--addr", fmt.Sprintf("localhost:%s", *waterfallServicePort)})

	if err != nil {
		log.Fatalf("Failed to start fork daemon: %s, %v", out, err)
	}

	out, err = adbConn.Shell([]string{"getprop", sdkProp})
	if err != nil {
		log.Fatalf("Failed to get device API level: %v", err)
	}

	v, err := strconv.Atoi(out)
	if err != nil {
		log.Fatalf("Failed to parse device API level: %s %v", out, err)
	}

	// Starting with Oreo starting foreground services requires a different command.
	commonArgs := []string{
		"-a", "proxy", "-n", fmt.Sprintf("%s/.UsbService", usbPackageName), "--ei", "waterfallPort", *waterfallServicePort}
	args := []string{"am"}
	if v >= 26 {
		args = append(args, "start-foreground-service")
	} else {
		args = append(args, "startservice")
	}
	args = append(args, commonArgs...)

	if _, err := adbConn.Shell(args); err != nil {
		log.Fatalf("Faile to start USB service: %v", err)
	}
	time.Sleep(500 * time.Millisecond)

	cmd := exec.Command(*forwarderBin, "-connect_addr", fmt.Sprintf("mux:usb:%s", *adbDeviceSerial), "-listen_addr", devAddr)
	if err := cmd.Start(); err != nil {
		log.Fatal(err)
	}
}
