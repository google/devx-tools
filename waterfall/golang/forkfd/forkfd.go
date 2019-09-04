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

var (
	addr           = flag.String("addr", "localhost:8080", "Address to listen on.")
	binaryToLaunch = flag.String("binary_to_launch", "", "Path to the binary to launch")
)

// launchProcess launches binary dupping the conn fd and making it accessible to the forked process.
func launchProcess(binaryPath string, rc syscall.RawConn, passthroughArgs []string) error {
	var cmd *exec.Cmd
	var cmdErr error

	args := append([]string{"-addr", fmt.Sprintf("mux:%d", 3)}, passthroughArgs...)
	err := rc.Control(func(fd uintptr) {
		log.Printf("Forking %s with fd: %d", binaryPath, fd)
		f := os.NewFile(fd, "")
		cmd = exec.Command(binaryPath, args...)
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

		if err := launchProcess(*binaryToLaunch, rawConn, flag.Args()); err != nil {
			// just wait for the next connection on error
			log.Printf("Error running process: %v\n", err)
		}
	}
}
