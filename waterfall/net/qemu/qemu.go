// Package qemu provides implementations of a net.Conn and net.Listener backed by qemu_pipe
package qemu

import (
	"encoding/binary"
	"errors"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	// The name of the socket that will be used by the qemu_pipe.
	// The name of the socket is passed to the emulator at startup time and can't change.
	SocketName = "sockets/h2o"

	qemuDriver = "/dev/qemu_pipe"
	svcName    = "pipe:unix:sockets/h2o"
	ioErrMsg   = "input/output error"
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
	return svcName
}

// Pipe implements a net.Listener on top of a guest qemu pipe
type Pipe struct {
	closed bool
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

	readLock  *sync.Mutex
	writeLock *sync.Mutex
}

// Read reads from from the Conn connection.
// Note that each message is prepended with the size, so we need to keep
// track of how many bytes we need to read across Read calls
func (q *Conn) Read(b []byte) (int, error) {
	q.readLock.Lock()
	defer q.readLock.Unlock()

	if q.closedReads {
		return 0, errClosed
	}

	toRead := q.left
	if toRead > len(b) {
		toRead = len(b)
	}

	// read leftovers from previous reads
	ln := toRead
	if q.left > 0 {
		if n, err := io.ReadFull(q.conn, b[:toRead]); err != nil {
			return n, err
		}
		// If ReadFull finished without issues, exactly toRead bytes were read
		q.left = q.left - toRead

		if q.left > 0 || toRead == len(b) {
			return toRead, nil
		}
	}

	var recd int32
	if err := binary.Read(q.conn, binary.LittleEndian, &recd); err != nil {
		return 0, err
	}

	// The other side has signaled an EOF. Close connection for reads, we might have leftover writes
	if recd == 0 {
		q.closedReads = true
		if q.closedWrites {
			// No more to read or write, close the pipe
			q.conn.Close()
		}
		return ln, io.EOF
	}

	sCap := len(b) - ln
	toRead = int(recd)
	if toRead > sCap {
		toRead = sCap
	}
	b = b[ln : ln+toRead]

	rn, err := io.ReadFull(q.conn, b)
	if err != nil {
		return rn, err
	}
	q.left = int(recd) - toRead
	return ln + rn, nil
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
	n, e := q.conn.Write(b)
	return n, e
}

func (q *Conn) sendClose() error {
	if err := binary.Write(q.conn, binary.LittleEndian, uint32(0)); err != nil {
		return err
	}
	_, e := q.conn.Write([]byte{})
	return e
}

// Close closes the connection
func (q *Conn) Close() error {
	q.writeLock.Lock()
	defer q.writeLock.Unlock()

	if q.closedWrites {
		return errClosed
	}
	q.closedWrites = true
	if err := q.sendClose(); err != nil {
		return err
	}
	if q.closedReads {
		// Nothing else to write or read, close the pipe
		if err := q.conn.Close(); err != nil {
			return err
		}
	}
	return nil
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

// MakeConn creates a net.Conn backed by a net.Listener.
// When using qemu_pipe connections the host (client) is the
// one responsible for opening the connection.
func MakeConn(lis net.Listener) (net.Conn, error) {
	conn, err := lis.Accept()
	if err != nil {
		return nil, err
	}

	// sync with the server
	if _, err := conn.Write([]byte("rdy")); err != nil {
		return nil, err
	}

	return &Conn{
		conn:      conn,
		addr:      qemuAddr(""),
		readLock:  &sync.Mutex{},
		writeLock: &sync.Mutex{},
	}, nil

}

// Accept creates a new net.Conn backed by a qemu_pipe connetion
func (q *Pipe) Accept() (net.Conn, error) {

	if q.closed {
		return nil, errClosed
	}

	// Each new file descriptor we open will create a new connection
	// We need to wait on the host to be ready:
	// 1) poll the qemu_pipe driver with the desiered socket name
	// 2) wait until the client is read to send/recv, we do this waiting until we read a rdy message
	var conn *os.File
	br := false
	for !br {
		var err error
		if conn, err = os.OpenFile(qemuDriver, os.O_RDWR, 0600); err != nil {
			return nil, err
		}

		// qemu_pipe does not properly support polling io. Force blocking io
		if err := syscall.SetNonblock(int(conn.Fd()), false); err != nil {
			return nil, err
		}

		buff := make([]byte, len(svcName)+1)
		copy(buff, svcName)

		// retry loop to wait until we can start the service on the qemu_pipe
		for {
			written, err := conn.Write(buff)
			if err != nil {
				// The host has not opened the socket. Sleep and try again
				conn.Close()
				time.Sleep(100 * time.Millisecond)
				break
			}
			buff = buff[written:]
			if len(buff) == 0 {
				br = true
				break
			}
		}

		if br {

			// Wait for the client to open a socket on the host
			waitBuff := make([]byte, 3)
			if _, err := io.ReadFull(conn, waitBuff); err != nil {
				if strings.Contains(err.Error(), ioErrMsg) {
					conn.Close()
					time.Sleep(100 * time.Millisecond)
					br = false
				} else {
					return nil, err
				}
			}
		}
	}

	return &Conn{
		conn:      conn,
		addr:      qemuAddr(""),
		readLock:  &sync.Mutex{},
		writeLock: &sync.Mutex{},
	}, nil
}

// Close closes the connection
func (q *Pipe) Close() error {
	q.closed = true
	return nil
}

// Addr returns the connection address
func (q *Pipe) Addr() net.Addr {
	return qemuAddr("")
}

// MakePipe will return a new net.Listener
// backed by a qemu pipe. Qemu pipes are implemented as virtual
// devices. To get a handle an open("/dev/qemu_pipe") is issued.
// The virtual driver keeps a map of file descriptors to available
// services. In this case we open a unix socket service and return that.
func MakePipe() (*Pipe, error) {
	if _, err := os.Stat(qemuDriver); err != nil {
		return nil, err
	}

	return &Pipe{}, nil
}
