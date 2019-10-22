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

// forkfd listens for tcp and spawns a waterfall server receiving on the opened connection.
package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"syscall"
)

const (
	// Path to logwrapper binary on device. This pipes stdout/stderr to logcat.
	logwrapperBin = "/system/bin/logwrapper"
)

var (
	binaryToLaunch = flag.String("binary_to_launch", "", "Path to the binary to launch.")
	fork = flag.Bool("fork", false, "Weather to listen on -addr and dup the conn fd to pass it to the forked process.")
	addr           = flag.String("addr", "localhost:8080", "Address to listen on for connections to pass to forked process.")
)

// launchAndDupConn launches a binary dupping the conn fd and making it accessible to the forked process.
func launchAndDupConn(binaryPath string, rc syscall.RawConn, passthroughArgs []string) error {
	var cmd *exec.Cmd
	var cmdErr error

	args := append([]string{binaryPath, "-addr", fmt.Sprintf("mux:fd:%d", 3)}, passthroughArgs...)
	err := rc.Control(func(fd uintptr) {
		log.Printf("Forking %s with fd: %d\n", binaryPath, fd)
		f := os.NewFile(fd, "")
		cmd = exec.Command(logwrapperBin, args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.ExtraFiles = append(cmd.ExtraFiles, f)
		cmdErr = cmd.Start()
	})

	if err != nil {
		return err
	}

	if cmdErr != nil {
		return err
	}

	return cmd.Wait()
}

func main() {
	flag.Parse()

	if *addr == "" {
		log.Fatalf("Need to provide -addr.")
	}

	if *binaryToLaunch == "" {
		log.Fatalf("Need to provide -binary_to_launch.")
	}

	if *fork {
		log.Printf("Launching %s\n", *binaryToLaunch)
		cmd := exec.Command(os.Args[0], "-addr", *addr, "-binary_to_launch", *binaryToLaunch)
		if err := cmd.Start(); err != nil {
			log.Fatal(err)
		}
		return
	}

	lis, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	defer lis.Close()

	for {
		log.Printf("Listening on %s\n", *addr)
		conn, err := lis.Accept()
		if err != nil {
			log.Fatalf("Failed to accept: %v", err)
		}
		defer conn.Close()

		log.Println("Accepted connection. Starting process ...")

		rawConn, err := conn.(*net.TCPConn).SyscallConn()
		if err != nil {
			log.Fatalf("Failed to get raw conn: %v", err)
		}

		if err := launchAndDupConn(*binaryToLaunch, rawConn, flag.Args()); err != nil {
			// just wait for the next connection on error
			log.Printf("Error running process: %v\n", err)
		}
	}
}
