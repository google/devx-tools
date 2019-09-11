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

// Package qemu provides implementations of a net.Conn and net.Listener backed by qemu_pipe
package qemu

import (
	"bufio"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	control_socket_pb "github.com/google/waterfall/proto/control_socket_go_proto"
)

// ControlSocketConn implements the net.Conn interface on top of a qemu_pipe, communicating open,close events over the control socket.
type ControlSocketConn struct {
	// Backed by qemu_pipe
	wb *bufio.Writer
	rb *bufio.Reader
	cl io.Closer

	cb *controlSocket
	fd uint32

	// Dummy addr
	addr net.Addr

	closedReads  bool
	closedWrites bool
	closed       bool

	readsCloseChan  chan struct{}
	writesCloseChan chan struct{}

	readLock  *sync.Mutex
	writeLock *sync.Mutex
	closeLock *sync.Mutex
}

// Read implements the standard Read interface.
func (q *ControlSocketConn) Read(b []byte) (int, error) {
	q.readLock.Lock()
	defer q.readLock.Unlock()
	return q.rb.Read(b)
}

// Write implements the standard Write interface.
func (q *ControlSocketConn) Write(b []byte) (int, error) {
	q.writeLock.Lock()
	defer q.writeLock.Unlock()

	if q.closedWrites {
		return 0, errClosed
	}

	n, err := q.wb.Write(b)
	if err != nil {
		return n, err
	}
	return n, q.wb.Flush()
}

// Close closes the connection, this will send out a close event on the control socket.
func (q *ControlSocketConn) Close() error {
	q.closeLock.Lock()
	defer q.closeLock.Unlock()

	if q.closed {
		return errClosed
	}

	q.closed = true
	q.CloseWrite()
	q.CloseRead()
	q.cl.Close()
	q.cb.close(q)
	return nil
}

// CloseRead closes the read side of the connection
func (q *ControlSocketConn) CloseRead() error {
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
func (q *ControlSocketConn) CloseWrite() error {
	q.writeLock.Lock()
	defer q.writeLock.Unlock()

	if q.closedWrites == true {
		return errClosed
	}
	q.closedWrites = true

	close(q.writesCloseChan)
	return nil
}

// LocalAddr returns the qemu address
func (q *ControlSocketConn) LocalAddr() net.Addr {
	return q.addr
}

// RemoteAddr returns the qemu address
func (q *ControlSocketConn) RemoteAddr() net.Addr {
	return q.addr
}

// SetDeadline sets the connection deadline
func (q *ControlSocketConn) SetDeadline(t time.Time) error {
	return errNotImplemented
}

// SetReadDeadline sets the read deadline
func (q *ControlSocketConn) SetReadDeadline(t time.Time) error {
	return errNotImplemented
}

// SetWriteDeadline sets the write deadline
func (q *ControlSocketConn) SetWriteDeadline(t time.Time) error {
	return errNotImplemented
}

// closeConn waits for the read end and the write end of the connection to be closed and then closes the underlying connection
func (q *ControlSocketConn) closeConn() {
	// remember the ABC
	<-q.readsCloseChan
	<-q.writesCloseChan
	q.cl.Close()
}

func makeControlSocketConn(conn io.ReadWriteCloser, fd uint32, cs *controlSocket) *ControlSocketConn {
	return &ControlSocketConn{
		wb:              bufio.NewWriterSize(conn, buffSize),
		rb:              bufio.NewReaderSize(conn, buffSize),
		cl:              conn,
		cb:              cs,
		fd:              fd,
		addr:            qemuAddr(""),
		readsCloseChan:  make(chan struct{}),
		writesCloseChan: make(chan struct{}),
		readLock:        &sync.Mutex{},
		writeLock:       &sync.Mutex{},
		closeLock:       &sync.Mutex{},
	}
}

// Message indicating that a connection has become available, or an error arose during connecting.
type availableSocket struct {
	c   net.Conn // The connection that became available
	err error    // The error if any during opening.
}

// ControlSocket is a socket that uses one channel to control if a socket should be opend or closed.
type controlSocket struct {
	pipe      *Pipe
	m         sync.Map             // VirtualFd -> ControlSocketConn
	available chan availableSocket // Channel with available socket/error (net.Conn, error)
	c         net.Conn             // Control socket used for open/close communication.
}

// Close closes out the control socket.
func (b *controlSocket) Close() error {
	// Close out the control socket first.
	b.c.Close()
	return b.pipe.Close()
}

// Close down the socket by sending a close message over the control channel to the host.
func (b *controlSocket) close(q *ControlSocketConn) error {
	b.m.Delete(q.fd)
	if q.fd == 0 {
		// The control socket vanished. We close out every connection
		// as we have no way of knowing whether they are active or not.
		b.m.Range(
			func(key, value interface{}) bool {
				value.(net.Conn).Close()
				return true
			})
	}

	cl := control_socket_pb.SocketControl_close
	cinfo := &control_socket_pb.SocketControl{
		Fd:   &q.fd,
		Sort: &cl,
	}

	out, err := proto.Marshal(cinfo)
	if err != nil {
		return err
	}
	_, err = b.c.Write(out)
	return err
}

// Open opens the socket with the given virtual fd. The connection and errors will be posted back on the available channel.
func (b *controlSocket) Open(fd uint32) (net.Conn, error) {
	c, err := b.open(fd)
	socket := availableSocket{c, err}
	b.available <- socket
	return c, err
}

// open opens a connection without blocking by calling accept and sending the identity message over the connection.
func (b *controlSocket) open(fd uint32) (net.Conn, error) {
	cl := control_socket_pb.SocketControl_identity
	cinfo := &control_socket_pb.SocketControl{
		Fd:   &fd,
		Sort: &cl,
	}

	// Block and wait until a connection has been made.
	base, err := b.pipe.Accept()
	conn := makeControlSocketConn(base, fd, b)
	go conn.closeConn()

	if err != nil {
		log.Printf("Failed to open connection to %d due to %s", fd, err)
		return conn, err
	}

	out, err := proto.Marshal(cinfo)
	if err != nil {
		log.Printf("Failed to marshall identity announcement for %d due to %s", fd, err)
		return conn, err
	}

	_, err = conn.Write(out)
	if err != nil {
		log.Printf("Failed to send identity message for %d due to %s", fd, err)
		return conn, err
	}

	// We are opening the control socket..
	if fd == 0 {
		b.c = conn
		log.Printf("Openend the control socket")
		return conn, nil
	}

	c := makeControlSocketConn(conn, fd, b)
	b.m.Store(fd, c)

	return c, err
}

// Blocks and waits until a new connection/error has been made available.
func (b *controlSocket) nextConnection() (net.Conn, error) {
	available := <-b.available
	return available.c, available.err
}

func (b *controlSocket) controlSocketHandler() error {
	defer b.c.Close()
	for {
		// Note! It's very important that the control socket
		// has fixed size! Don't mess with variable encoding lest you want to make this all complicated.
		var fd uint32 = 0
		cl := control_socket_pb.SocketControl_identity
		cinfo := &control_socket_pb.SocketControl{
			Fd:   &fd,
			Sort: &cl,
		}
		cBytes := make([]byte, cinfo.XXX_Size())
		_, err := io.ReadFull(b.c, cBytes)
		if err != nil {
			return err
		}

		if err := proto.Unmarshal(cBytes, cinfo); err != nil {
			log.Printf("Failed to unmarshall socket message due to %s", err)
			return err
		}
		// There are basically 2 options
		// 1. A new connection please..
		// 2. Close out a connection please.
		switch sort := cinfo.GetSort(); sort {
		case control_socket_pb.SocketControl_open:
			// Add a new connection, note that errors are send over the channel.
			log.Printf("Opening a connection for virtual fd: %d", cinfo.GetFd())
			go b.Open(cinfo.GetFd())
		case control_socket_pb.SocketControl_close:
			log.Print("Virtual fd: %d has closed", cinfo.GetFd())
			if con, ok := b.m.Load(cinfo.GetFd()); ok {
				// Invoke close, call back will happen to clean up later
				con.(net.Conn).Close()
			}
		case control_socket_pb.SocketControl_identity:
			// Excuse me? why are you sending this on the control socket?
		}
	}
}

// Accepts a new incoming connection. Requests for new connections have been communicated over the control channel.
func (b *controlSocket) Accept() (net.Conn, error) {
	if b.c == nil {
		conn, err := b.open(0)
		if err != nil {
			return conn, err
		}
		b.c = conn
		go b.controlSocketHandler()
	}

	// Block and wait for the next connection..
	nxt, err := b.nextConnection()
	return nxt, err
}

// Addr returns the connection address
func (q *controlSocket) Addr() net.Addr {
	return qemuAddr(q.pipe.socketName)
}

// Creates a new control socket that communicates over the given pipe.
func MakeControlSocket(pipe *Pipe) *controlSocket {
	s := controlSocket{
		pipe:      pipe,
		available: make(chan availableSocket),
	}
	return &s
}
