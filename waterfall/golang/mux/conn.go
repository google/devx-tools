package mux

import (
	"fmt"
	"io"
	"net"
	"time"
)

type halfCloser interface {
	CloseRead() error
	CloseWrite() error
}

// Conn implements net.Conn interface on a ReadWriteCloser.
type Conn struct {
	io.ReadWriteCloser
}

// Read reads from the underlying reader.
func (mc *Conn) Read(b []byte) (int, error) {
	return mc.ReadWriteCloser.Read(b)
}

// Write writes to the underlying writer.
func (mc *Conn) Write(b []byte) (int, error) {
	return mc.ReadWriteCloser.Write(b)
}

// Close closes the underlying closer.
func (mc *Conn) Close() error {
	return mc.ReadWriteCloser.Close()
}

// CloseRead closes the read side of the ReadWriteCloser if it implements CloseRead,
// otherwise it closes the closer.
func (mc *Conn) CloseRead() error {
	if c, ok := mc.ReadWriteCloser.(halfCloser); ok {
		return c.CloseRead()
	}
	return mc.ReadWriteCloser.Close()
}

// CloseWrite closes the write side of the ReadWriteCloser if it implements CloseWrite,
// otherwise it closes the closer.
func (mc *Conn) CloseWrite() error {
	if c, ok := mc.ReadWriteCloser.(halfCloser); ok {
		return c.CloseWrite()
	}
	return mc.ReadWriteCloser.Close()
}

// LocalAddr returns the conn addr.
func (mc *Conn) LocalAddr() net.Addr {
	return maddr("muxconn")
}

// RemoteAddr returns the conn add.
func (mc *Conn) RemoteAddr() net.Addr {
	return maddr("muxconn")
}

// SetDeadline -> unimplemented, calling it returns an error.
func (mc *Conn) SetDeadline(t time.Time) error {
	return fmt.Errorf("unimplemented SetDeadline")
}

// SetReadDeadline -> unimplemented, calling it returns an error.
func (mc *Conn) SetReadDeadline(t time.Time) error {
	return fmt.Errorf("unimplemented SetReadDeadline")
}

// SetWriteDeadline -> unimplemented, calling it returns an error.
func (mc *Conn) SetWriteDeadline(t time.Time) error {
	return fmt.Errorf("unimplemented SetWriteDeadline")
}

// NewConn returns a new Conn wrapping the ReadWriteCloser.
func NewConn(rwc io.ReadWriteCloser) net.Conn {
	return &Conn{rwc}
}
