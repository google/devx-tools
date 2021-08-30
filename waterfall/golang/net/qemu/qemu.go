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

// Package qemu provides implementations of a net.Conn and net.Listener backed by qemu_pipe
package qemu

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	qemuDriver = "/dev/qemu_pipe"
	qemuSvc    = "pipe:unix:"
	ioErrMsg   = "input/output error"
	rdyMsg     = "rdy"

	// We pick a sufficiently large buffer size to avoid hitting an underlying bug on the emulator.
	// See https://issuetracker.google.com/issues/115894209 for context.
	buffSize = 1 << 16
)

var errClosed = errors.New("error connection closed")
var errNotImplemented = errors.New("error not implemented")

type qemuAddr string

// Network returns the network type of the connection
func (a qemuAddr) Network() string {
	return qemuDriver
}

// String returns the description of the connection
func (a qemuAddr) String() string {
	return string(a)
}

// Conn implements the net.Conn interface on top of a qemu_pipe
type Conn struct {
	// Backed by qemu_pipe
	wb *bufio.Writer
	rb *bufio.Reader
	cl io.Closer

	// Dummy addr
	addr net.Addr

	// Bytes remaining in the connection buffer
	// it would be tempting to simplify the code and use
	// an intermediate buffer to hold overflow buffer
	// but bytes.Buffer does not shrink which causes memory usage
	// to blow up
	left int

	closedReads  bool
	closedWrites bool
	closed       bool

	readsCloseChan  chan struct{}
	writesCloseChan chan struct{}

	readLock  *sync.Mutex
	writeLock *sync.Mutex
	closeLock *sync.Mutex
}

// Read reads from from the Conn connection.
// Note that each message is prepended with the size, so we need to keep
// track of how many bytes we need to read across Read calls
func (q *Conn) Read(b []byte) (int, error) {
	q.readLock.Lock()
	defer q.readLock.Unlock()

	// Normally this could be done by just checking status != io.EOF, however
	// a read on /dev/qemu-pipe associated with a closed connection will return
	// EIO with the string "input/output error". Since errno isn't visible to us,
	// we check the error message and ensure the target string is not present.
	n, err := q.read(b)
	if err != nil {
		if strings.Contains(err.Error(), ioErrMsg) {
			return n, io.EOF
		}
	}
	return n, err

}

func (q *Conn) read(b []byte) (int, error) {
	if q.closedReads {
		return 0, errClosed
	}

	toRead := q.left
	if toRead > len(b) {
		toRead = len(b)
	}

	// read leftovers from previous reads before trying to read the size
	if toRead > 0 {
		n, err := io.ReadFull(q.rb, b[:toRead])
		if err != nil {
			return n, err
		}
		q.left = q.left - n

		// b might be bigger than remaining bytes but conn might be empty and
		// we dont want to block on size read, so return early.
		return n, nil
	}

	var recd uint32
	if err := binary.Read(q.rb, binary.LittleEndian, &recd); err != nil {
		return 0, err
	}

	// The other side has signaled an EOF. Close connection for reads, we might have leftover writes
	if recd == 0 {
		return 0, io.EOF
	}

	toRead = int(recd)
	if toRead > len(b) {
		toRead = len(b)
	}

	n, err := io.ReadFull(q.rb, b[:toRead])
	if err != nil {
		return n, err
	}
	q.left = int(recd) - toRead
	return n, nil
}

// Write writes the contents of b to the underlying connection
func (q *Conn) Write(b []byte) (int, error) {
	q.writeLock.Lock()
	defer q.writeLock.Unlock()

	if q.closedWrites {
		return 0, errClosed
	}

	// Prepend the message with size
	if err := binary.Write(q.wb, binary.LittleEndian, uint32(len(b))); err != nil {
		return 0, err
	}
	n, err := q.wb.Write(b)
	if err != nil {
		return n, err
	}
	return n, q.wb.Flush()
}

// Close closes the connection
func (q *Conn) Close() error {
	q.closeLock.Lock()
	defer q.closeLock.Unlock()

	if q.closed {
		return errClosed
	}

	q.closed = true

	q.CloseWrite()
	q.CloseRead()
	q.cl.Close()
	return nil
}

// CloseRead closes the read side of the connection
func (q *Conn) CloseRead() error {
	q.readLock.Lock()
	defer q.readLock.Unlock()

	if q.closedReads {
		return errClosed
	}
	q.closedReads = true
	close(q.readsCloseChan)
	return nil
}

// CloseWrite closes the write side of the connection
func (q *Conn) CloseWrite() error {
	q.writeLock.Lock()
	defer q.writeLock.Unlock()

	if q.closedWrites == true {
		return errClosed
	}
	q.closedWrites = true

	err := q.sendClose()
	close(q.writesCloseChan)
	return err
}

func (q *Conn) sendClose() error {
	if err := binary.Write(q.wb, binary.LittleEndian, uint32(0)); err != nil {
		return err
	}
	if _, err := q.wb.Write([]byte{}); err != nil {
		return err
	}
	return q.wb.Flush()
}

// LocalAddr returns the qemu address
func (q *Conn) LocalAddr() net.Addr {
	return q.addr
}

// RemoteAddr returns the qemu address
func (q *Conn) RemoteAddr() net.Addr {
	return q.addr
}

// SetDeadline sets the connection deadline
func (q *Conn) SetDeadline(t time.Time) error {
	return errNotImplemented
}

// SetReadDeadline sets the read deadline
func (q *Conn) SetReadDeadline(t time.Time) error {
	return errNotImplemented
}

// SetWriteDeadline sets the write deadline
func (q *Conn) SetWriteDeadline(t time.Time) error {
	return errNotImplemented
}

// closeConn waits for the read end and the write end of th connection
// to be closed and then closes the underlying connection
func (q *Conn) closeConn() {
	// remember the ABC
	<-q.readsCloseChan
	<-q.writesCloseChan
	q.cl.Close()
}

func makeConn(conn io.ReadWriteCloser) *Conn {
	return &Conn{
		wb:              bufio.NewWriterSize(conn, buffSize),
		rb:              bufio.NewReaderSize(conn, buffSize),
		cl:              conn,
		addr:            qemuAddr(""),
		readsCloseChan:  make(chan struct{}),
		writesCloseChan: make(chan struct{}),
		readLock:        &sync.Mutex{},
		writeLock:       &sync.Mutex{},
		closeLock:       &sync.Mutex{},
	}
}

// ConnBuilder implements a qemu connection builder. It wraps around listener
// listening on a qemu pipe. It accepts connectsion and sync with the client
// before returning
type ConnBuilder struct {
	net.Listener
}

// Accept will connect to the guest and return the connection.
func (b *ConnBuilder) Accept() (net.Conn, error) {
	for {
		conn, err := b.Listener.Accept()
		if err != nil {
			return nil, err
		}

		// sync with the server
		rdy := []byte(rdyMsg)
		if _, err := conn.Write(rdy); err != nil {
			conn.Close()
			continue
		}
		if _, err := io.ReadFull(conn, rdy); err != nil {
			conn.Close()
			continue
		}
		if !bytes.Equal([]byte(rdyMsg), rdy) {
			conn.Close()
			continue
		}

		q := makeConn(conn)

		go q.closeConn()
		return q, nil
	}
}

// MakeConnBuilder creates a new ConnBuilder struct
func MakeConnBuilder(emuDir, socket string) (*ConnBuilder, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("cannot get working directory: %v", err)
	}

	if wd != emuDir {
		if err := os.Chdir(emuDir); err != nil {
			return nil, fmt.Errorf("failed to chdir to %s: %v", emuDir, err)
		}
	}

	if err := os.Remove(socket); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to remove old socket: %v", err)
	}
	lis, err := net.Listen("unix", socket)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on unix socket %s: %v", socket, err)
	}

	if wd != emuDir {
		if err := os.Chdir(wd); err != nil {
			return nil, fmt.Errorf("failed to chdir to %s: %v", wd, err)
		}
	}

	return &ConnBuilder{Listener: lis}, nil
}

// PipeConnBuilder implements a qemu connection builder. It wraps around listener
// listening on a qemu pipe. It accepts connectsion and sync with the client
// before returning
type PipeConnBuilder struct {
	net.Listener
}

// Accept will connect to the guest and return the connection.
func (b *PipeConnBuilder) Accept() (net.Conn, error) {
	for {
		conn, err := b.Listener.Accept()
		if err != nil {
			return nil, err
		}

		// sync with the server
		rdy := []byte(rdyMsg)
		if _, err := io.ReadFull(conn, rdy); err != nil {
			conn.Close()
			continue
		}
		if !bytes.Equal([]byte(rdyMsg), rdy) {
			conn.Close()
			continue
		}
		if _, err := conn.Write(rdy); err != nil {
			conn.Close()
			continue
		}

		q := makeConn(conn)

		go q.closeConn()
		return q, nil
	}
}

// MakePipeConnBuilder returns a PipeConuilder using pipe.
func MakePipeConnBuilder(pipe *Pipe) *PipeConnBuilder {
	return &PipeConnBuilder{Listener: pipe}
}

// QemuConn implements a ReadWriteInterface with a qemu pipe.
type QemuConn struct {
	io.ReadWriteCloser
}

// MakeQemuConn create new qemu control socket backed by a qemu pipe file.
func MakeQemuConn(file *os.File) *QemuConn {
	return &QemuConn{ReadWriteCloser: file}
}

// LocalAddr returns the qemu address
func (q *QemuConn) LocalAddr() net.Addr {
	return &net.UnixAddr{Name: "qemu-pipe"}
}

// RemoteAddr returns the qemu address
func (q *QemuConn) RemoteAddr() net.Addr {
	return &net.UnixAddr{Name: "qemu-pipe"}
}

// SetDeadline sets the connection deadline
func (q *QemuConn) SetDeadline(t time.Time) error {
	return errNotImplemented
}

// SetReadDeadline sets the read deadline
func (q *QemuConn) SetReadDeadline(t time.Time) error {
	return errNotImplemented
}

// SetWriteDeadline sets the write deadline
func (q *QemuConn) SetWriteDeadline(t time.Time) error {
	return errNotImplemented
}

func openQemuDevBlocking() (*os.File, error) {
	// Open device manually in blocking mode. Qemu pipes don't support polling io.
	r, err := syscall.Open(qemuDriver, os.O_RDWR|syscall.O_CLOEXEC, 0600)
	if err != nil {
		return nil, err
	}

	return os.NewFile(uintptr(r), qemuDriver), nil
}

// Pipe implements a net.Listener on top of a guest qemu pipe
type Pipe struct {
	socketName string
	closed     bool
}

// Accept creates a new net.Conn backed by a qemu_pipe connetion
func (q *Pipe) Accept() (net.Conn, error) {

	log.Println("Creating new conn")
	if q.closed {
		return nil, errClosed
	}

	// Each new file descriptor we open will create a new connection
	// We need to wait on the host to be ready:
	// 1) poll the qemu_pipe driver with the desiered socket name
	// 2) wait until the client is read to send/recv, we do this waiting until we read a rdy message
	var conn *os.File
	var err error
	br := false
	for {
		if conn != nil {
			// Got an error, close connection and try again
			conn.Close()
			time.Sleep(20 * time.Millisecond)
		}

		conn, err = openQemuDevBlocking()
		if err != nil {
			return nil, err
		}
		log.Println("File opened")

		svcName := qemuSvc + q.socketName
		buff := make([]byte, len(svcName)+1)
		copy(buff, svcName)

		// retry loop to wait until we can start the service on the qemu_pipe
		for {
			log.Println("Writing service")
			written, err := conn.Write(buff)
			if err != nil {
				// The host has not opened the socket. Sleep and try again
				conn.Close()
				time.Sleep(100 * time.Millisecond)
				break
			}
			buff = buff[written:]
			if len(buff) == 0 {
				log.Println("Wrote service")
				br = true
				break
			}
		}

		if br {
			return MakeQemuConn(conn), nil
		}
	}

}

// Close closes the connection
func (q *Pipe) Close() error {
	q.closed = true
	return nil
}

// Addr returns the connection address
func (q *Pipe) Addr() net.Addr {
	return qemuAddr(q.socketName)
}

// MakePipe will return a new net.Listener
// backed by a qemu pipe. Qemu pipes are implemented as virtual
// devices. To get a handle an open("/dev/qemu_pipe") is issued.
// The virtual driver keeps a map of file descriptors to available
// services. In this case we open a unix socket service and return that.
func MakePipe(socketName string) (*Pipe, error) {
	if _, err := os.Stat(qemuDriver); err != nil {
		return nil, err
	}

	return &Pipe{socketName: socketName}, nil
}
