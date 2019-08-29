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
	addr      = flag.String("addr", "localhost:8080", "Address to listen on.")
	waterfall = flag.String("waterfall", "/sbin/waterfall", "Path to the waterfall binary")
)

// launchWaterfallWithConn launches waterfall with a dupping the conn fd and making it accessible to the waterfall process
func runWaterfall(waterfallPath string, rc syscall.RawConn) error {
	var cmd *exec.Cmd
	var cmdErr error
	err := rc.Control(func(fd uintptr) {
		log.Printf("Forking waterfall with fd: %d", fd)
		f := os.NewFile(fd, "")
		cmd = exec.Command(waterfallPath, "-addr", fmt.Sprintf("mux:%d", 3))
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

	if *waterfall == "" {
		log.Fatalf("Need to provide -waterfall.")
	}

	lis, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	defer lis.Close()

	for {
		conn, err := lis.Accept()
		if err != nil {
			log.Fatalf("Failed to accept: %v", err)
		}
		defer conn.Close()

		rawConn, err := conn.(*net.TCPConn).SyscallConn()
		if err != nil {
			log.Fatalf("Failed to get raw conn: %v", err)
		}

		if err := runWaterfall(*waterfall, rawConn); err != nil {
			// just wait for the next connection on error
			log.Printf("Error running waterfall: %v\n", err)
		}
	}
}
