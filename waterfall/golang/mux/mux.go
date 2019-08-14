// Package mux multiplexes a connection using gRPC (http2) streams.
// The gRPC server is started on a connection that can't be shared.
// When the client connects to the server the connection is meant to be persistent.
// Once the connection is establised the client can call NewStream and get multiple
// streams on a single connection allowing to multiplex the base channel.
package mux

import (
	"context"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/google/waterfall/golang/stream"
	waterfall_grpc "github.com/google/waterfall/proto/waterfall_go_grpc"
	"google.golang.org/grpc"
)

// type server implements the waterfall grpc Multiplexer service.
type server struct {
	streamCh chan waterfall_grpc.Multiplexer_NewStreamServer
}

// singletonListener implements a net.Listener that guarantees Accept is only ever called once.
type singletonListener struct {
	conn   net.Conn
	used   bool
	closed bool

	// connCh only ever holds one connection.
	// Only first call to Accept succeed, any call after that will block on the channel.
	// Goroutines bloked on accept are only released after closed is called.
	connCh chan net.Conn
}

// Accept accepts a new connection if it has not accepted any connections yet.
func (sl *singletonListener) Accept() (net.Conn, error) {
	c, ok := <-sl.connCh
	if ok {
		return c, nil
	}

	return nil, fmt.Errorf("can't accept - listener closed")
}

// Close closes the listener.
func (sl *singletonListener) Close() error {
	if sl.closed {
		return fmt.Errorf("alredy closed")
	}

	sl.closed = true
	close(sl.connCh)
	return sl.conn.Close()
}

func (sl *singletonListener) Addr() net.Addr {
	return maddr("singleton")
}

// NewStream only purpose is to get a handle to the gRPC stream and send it over the stream channel.
func (svr *server) NewStream(s waterfall_grpc.Multiplexer_NewStreamServer) error {
	svr.streamCh <- s

	// Block until the connection is closed
	<-s.Context().Done()

	return s.Context().Err()
}

// Listener implements net.Listener multiplexed over a gRPC connection
type Listener struct {
	control *singletonListener
	strms   chan waterfall_grpc.Multiplexer_NewStreamServer
	svr     *grpc.Server
}

// Close closes the Listener and stops the underlying gRPC server.
func (l *Listener) Close() error {
	if err := l.control.Close(); err != nil {
		return err
	}
	l.svr.Stop()
	close(l.strms)
	return nil
}

// Addr returns the listener address.
func (l *Listener) Addr() net.Addr {
	return maddr("mux")
}

// Accept returns the next available gRPC stream.
func (l *Listener) Accept() (net.Conn, error) {
	strm, ok := <-l.strms
	if !ok {
		return nil, fmt.Errorf("error receiving next stream: grpc server stopped")
	}

	return NewConn(stream.NewReadWriteCloser(strm, &Message{})), nil
}

// NewListener returns a Listener backed by gRPC service.
func NewListener(f io.ReadWriteCloser) *Listener {
	c := NewConn(f)
	sl := &singletonListener{
		conn:   NewConn(f),
		connCh: make(chan net.Conn, 1),
	}
	sl.connCh <- c

	ss := make(chan waterfall_grpc.Multiplexer_NewStreamServer)
	gsvr := grpc.NewServer()
	mux := &server{streamCh: ss}
	waterfall_grpc.RegisterMultiplexerServer(gsvr, mux)

	go func() {
		gsvr.Serve(sl)
	}()

	return &Listener{
		control: sl,
		strms:   ss,
		svr:     gsvr,
	}
}

// ConBuilder create new connections multiplexed throug gRPC
type ConnBuilder struct {
	ctx    context.Context
	client waterfall_grpc.MultiplexerClient
	conn   *grpc.ClientConn
}

// NewConnBuilder creates a ConnBuilder using a base ReadWriteCloser
func NewConnBuilder(ctx context.Context, rwc io.ReadWriteCloser) (*ConnBuilder, error) {
	r := NewConn(rwc)
	d := func(string, time.Duration) (net.Conn, error) {
		// TODO(mauriciogg): fail if a more than one dial happens.
		return r, nil
	}

	cc, err := grpc.Dial("", grpc.WithDialer(d), grpc.WithInsecure())
	return &ConnBuilder{
		ctx:    ctx,
		client: waterfall_grpc.NewMultiplexerClient(cc),
		conn:   cc,
	}, err
}

// Accept returns a new Conn.
func (sb *ConnBuilder) Accept() (net.Conn, error) {
	s, err := sb.client.NewStream(sb.ctx)
	if err != nil {
		return nil, err
	}

	return NewConn(stream.NewReadWriteCloser(s, Message{})), nil
}

// Close closes ConnBuilder.
func (sb *ConnBuilder) Close() error {
	return sb.conn.Close()
}
