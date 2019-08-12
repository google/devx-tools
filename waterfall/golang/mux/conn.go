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

type rwcConn struct {
	io.ReadWriteCloser
}

func (mc *rwcConn) Read(b []byte) (int, error) {
	return mc.ReadWriteCloser.Read(b)
}

func (mc *rwcConn) Write(b []byte) (int, error) {
	return mc.ReadWriteCloser.Write(b)
}

func (mc *rwcConn) Close() error {
	return mc.ReadWriteCloser.Close()
}

func (mc *rwcConn) CloseRead() error {
	if c, ok := mc.ReadWriteCloser.(halfCloser); ok {
		return c.CloseRead()
	}
	return mc.ReadWriteCloser.Close()
}

func (mc *rwcConn) CloseWrite() error {
	if c, ok := mc.ReadWriteCloser.(halfCloser); ok {
		return c.CloseWrite()
	}
	return mc.ReadWriteCloser.Close()
}

func (mc *rwcConn) LocalAddr() net.Addr {
	return maddr("muxconn")
}

func (mc *rwcConn) RemoteAddr() net.Addr {
	return maddr("muxconn")
}

func (mc *rwcConn) SetDeadline(t time.Time) error {
	return fmt.Errorf("unimplemented SetDeadline")
}

func (mc *rwcConn) SetReadDeadline(t time.Time) error {
	return fmt.Errorf("unimplemented SetReadDeadline")
}

func (mc *rwcConn) SetWriteDeadline(t time.Time) error {
	return fmt.Errorf("unimplemented SetWriteDeadline")
}
