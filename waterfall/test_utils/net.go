// Package test_utils has test utility functions for h2o modules
package test_utils

import (
	"errors"
	"math/rand"
	"net"
	"os"
	"sync"
	"syscall"
	"time"
)

var (
	rng = struct {
		sync.Mutex
		*rand.Rand
	}{Rand: rand.New(rand.NewSource(time.Now().UnixNano() + int64(syscall.Getpid())))}

	errNoFreePort = errors.New("no free port available")

	minPort = 32768
	maxPort = 65536
)

func isFree(port int) bool {
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, 0)
	if err != nil {
		return false
	}
	// Because of this callers need to bind to the port ASAP
	defer syscall.Close(fd)

	err = syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
	if err != nil {
		return false
	}

	if err := syscall.Bind(fd, &syscall.SockaddrInet4{Port: port}); err != nil {
		return false
	}
	return true
}

// pickUnusedPort returns a port number thats available to bind to.
func pickUnusedPort() (port int, err error) {
	// Start with random port in range [32768, 65536]
	rng.Lock()
	port = minPort + rng.Intn(maxPort-minPort+1)
	rng.Unlock()

	stop := port
	for {
		if isFree(port) {
			return port, nil
		}
		port++
		if port > maxPort {
			port = minPort
		}
		if port == stop {
			break
		}
	}

	return 0, errNoFreePort
}

func OpenSocket(dir, socketName string) (net.Listener, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	if err := os.Chdir(dir); err != nil {
		return nil, err
	}

	os.Remove(socketName)
	lis, err := net.Listen("unix", socketName)
	if err != nil {
		return nil, err
	}

	if err := os.Chdir(wd); err != nil {
		return nil, err
	}
	return lis, nil
}
