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
	"sync"
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
	mutex  *sync.Mutex
	active *sync.Mutex
}

// Accept accepts a new connection if it has not accepted any connections yet.
func (sl *singletonListener) Accept() (net.Conn, error) {
	sl.mutex.Lock()
	defer sl.mutex.Unlock()

	// TODO(mauriciogg): replace sync logic with a channel that only holds a value
	if sl.closed {
		return nil, fmt.Errorf("can't accept - listener closed")
	}

	if sl.used {
		// Wait until the connection is closed.
		// The pattern:
		// for {
		//   c, _ := lis.Accept()
		//   go handleConn(c)
		// }
		// Is very common in Go, if we return immediatly those cases will fail.
		// Insted block in order to simulate no more incoming requests.
		sl.active.Lock()
		return nil, fmt.Errorf("can't accept - listener used")
	}

	// Grab the lock and wait until closed is called. Only one active connection is allowed.
	sl.active.Lock()
	sl.used = true
	return sl.conn, nil
}

func (sl *singletonListener) Close() error {
	sl.mutex.Lock()
	defer sl.mutex.Unlock()

	if sl.closed {
		return nil
	}

	if sl.used {
		sl.active.Unlock()
	}

	sl.closed = true
	return sl.conn.Close()
}

func (sl *singletonListener) Addr() net.Addr {
	return maddr("singleton")
}

// NewStream sends the stream through the stream channel.
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

func (l *Listener) Close() error {
	if err := l.control.Close(); err != nil {
		return err
	}
	l.svr.Stop()
	close(l.strms)
	return nil
}

func (l *Listener) Addr() net.Addr {
	return maddr("mux")
}

func (l *Listener) Accept() (net.Conn, error) {
	strm, ok := <-l.strms
	if !ok {
		return nil, fmt.Errorf("error receiving next stream: grpc server stopped")
	}

	return NewConn(stream.NewReadWriteCloser(strm, &Message{})), nil
}

// NewListener returns a Listener backed multiplexed via a gRPC service built on top of the file descriptor.
func NewListener(f io.ReadWriteCloser) *Listener {
	sl := &singletonListener{
		conn:   NewConn(f),
		mutex:  &sync.Mutex{},
		active: &sync.Mutex{},
	}

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

// MakeStream returns a ReadWriteCloser backed by a grpc stream.
func (sb *ConnBuilder) Accept() (net.Conn, error) {
	s, err := sb.client.NewStream(sb.ctx)
	if err != nil {
		return nil, err
	}

	return NewConn(stream.NewReadWriteCloser(s, Message{})), nil
}

func (sb *ConnBuilder) Close() error {
	return sb.conn.Close()
}
