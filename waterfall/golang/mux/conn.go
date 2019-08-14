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

type Conn struct {
	io.ReadWriteCloser
}

func (mc *Conn) Read(b []byte) (int, error) {
	return mc.ReadWriteCloser.Read(b)
}

func (mc *Conn) Write(b []byte) (int, error) {
	return mc.ReadWriteCloser.Write(b)
}

func (mc *Conn) Close() error {
	return mc.ReadWriteCloser.Close()
}

func (mc *Conn) CloseRead() error {
	if c, ok := mc.ReadWriteCloser.(halfCloser); ok {
		return c.CloseRead()
	}
	return mc.ReadWriteCloser.Close()
}

func (mc *Conn) CloseWrite() error {
	if c, ok := mc.ReadWriteCloser.(halfCloser); ok {
		return c.CloseWrite()
	}
	return mc.ReadWriteCloser.Close()
}

func (mc *Conn) LocalAddr() net.Addr {
	return maddr("muxconn")
}

func (mc *Conn) RemoteAddr() net.Addr {
	return maddr("muxconn")
}

func (mc *Conn) SetDeadline(t time.Time) error {
	return fmt.Errorf("unimplemented SetDeadline")
}

func (mc *Conn) SetReadDeadline(t time.Time) error {
	return fmt.Errorf("unimplemented SetReadDeadline")
}

func (mc *Conn) SetWriteDeadline(t time.Time) error {
	return fmt.Errorf("unimplemented SetWriteDeadline")
}

func NewConn(rwc io.ReadWriteCloser) net.Conn {
	return &Conn{rwc}
}
