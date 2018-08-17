// Package qemu provides implementations of a net.Conn and net.Listener backed by qemu_pipe
package qemu

import (
	"bytes"
	"encoding/binary"
	"errors"
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
	conn io.ReadWriteCloser

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

	readsCloseChan  chan struct{}
	writesCloseChan chan struct{}

	readLock  *sync.Mutex
	writeLock *sync.Mutex
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
		go q.CloseRead()
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
		n, err := io.ReadFull(q.conn, b[:toRead])
		if err != nil {
			return n, err
		}
		q.left = q.left - n

		// b might be bigger than remaining bytes but conn might be empty and
		// we dont want to block on size read, so return early.
		return n, nil
	}

	var recd uint32
	if err := binary.Read(q.conn, binary.LittleEndian, &recd); err != nil {
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

	n, err := io.ReadFull(q.conn, b[:toRead])
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
	if err := binary.Write(q.conn, binary.LittleEndian, uint32(len(b))); err != nil {
		return 0, err
	}
	return q.conn.Write(b)
}

// Close closes the connection
func (q *Conn) Close() error {
	return q.CloseWrite()
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
	if err := binary.Write(q.conn, binary.LittleEndian, uint32(0)); err != nil {
		return err
	}
	_, e := q.conn.Write([]byte{})
	return e
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
	q.conn.Close()
}

func makeConn(conn io.ReadWriteCloser) *Conn {
	return &Conn{
		conn:            conn,
		addr:            qemuAddr(""),
		readsCloseChan:  make(chan struct{}),
		writesCloseChan: make(chan struct{}),
		readLock:        &sync.Mutex{},
		writeLock:       &sync.Mutex{},
	}
}

// ConnBuilder implements a qemu connection builder. It wraps around listener
// listening on a qemu pipe. It accepts connectsion and sync with the client
// before returning
type ConnBuilder struct {
	lis net.Listener
}

// Close closes the underlying net.Listener
func (b *ConnBuilder) Close() error {
	return b.lis.Close()
}

// Next will connect to the guest and return the connection.
func (b *ConnBuilder) Next() (net.Conn, error) {
	for {
		conn, err := b.lis.Accept()
		if err != nil {
			return nil, err
		}

		// sync with the server
		rdy := []byte(rdyMsg)
		if _, err := conn.Write(rdy); err != nil {
			continue
		}
		if _, err := io.ReadFull(conn, rdy); err != nil {
			continue
		}
		if !bytes.Equal([]byte(rdyMsg), rdy) {
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
		return nil, err
	}

	if err := os.Chdir(emuDir); err != nil {
		return nil, err
	}

	if err := os.Remove(socket); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	lis, err := net.Listen("unix", socket)
	if err != nil {
		return nil, err
	}

	if err := os.Chdir(wd); err != nil {
		return nil, err
	}

	return &ConnBuilder{lis: lis}, nil
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

		log.Println("Opening file")
		if conn, err = os.OpenFile(qemuDriver, os.O_RDWR, 0600); err != nil {
			return nil, err
		}
		log.Println("File opened")

		// qemu_pipe does not properly support polling io. Force blocking io
		if err := syscall.SetNonblock(int(conn.Fd()), false); err != nil {
			return nil, err
		}

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
			// Wait for the client to open a socket on the host
			waitBuff := make([]byte, len(rdyMsg))
			log.Println("Reading rdy")
			if _, err := io.ReadFull(conn, waitBuff); err != nil {
				br = false
				continue
			}
			if !bytes.Equal([]byte(rdyMsg), waitBuff) {
				br = false
				continue
			}
			log.Println("Writing rdy")
			if _, err := conn.Write(waitBuff); err != nil {
				br = false
				continue
			}

			log.Println("Done")
			q := makeConn(conn)

			go q.closeConn()
			return q, nil

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
