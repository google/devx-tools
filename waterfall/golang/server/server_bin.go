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

// Binary server_bin runs a waterfall gRPC server.
package main

import (
	"flag"
	"log"
	"syscall"

	"github.com/google/waterfall/golang/server"
)

var (
	addr = flag.String(
		"addr", "qemu:sockets/h2o", "Where to start listening. <qemu|tcp|unix>:addr."+
			" If qemu is specified, addr is the name of the pipe socket")

	cert       = flag.String("cert", "", "The path to the server certificate")
	privateKey = flag.String("private_key", "", "Path to the server private key")
)

func main() {
	flag.Parse()
	// do not chown - owners and groups will not be valid.
	// adb will always create files with 0644 permission
	syscall.Umask(0)

	if *addr == "" {
		log.Fatalf("Need to specify -addr.")
	}

	log.Println("Starting waterfall server ...")
	lis, err := server.WFListener(*addr)
	if err != nil {
		log.Fatalf("%v", err)
	}
	server, err := server.WFServer(*cert, *privateKey)
	if err != nil {
		log.Fatalf("%v", err)
	}

	log.Println("Serving ...")
	server.Serve(lis)
}
