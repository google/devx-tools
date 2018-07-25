package waterfall

import "io"

// Stream defines an interface to send and receive messages.
type Stream interface {
	SendMsg(m interface{}) error
	RecvMsg(m interface{}) error
}

// StreamMessage defines a generic interface to build messages with byte contents
type StreamMessage interface {
	BuildMsg() interface{}
	GetBytes(interface{}) ([]byte, error)
	SetBytes(interface{}, []byte)
	CloseMsg() interface{}
}

// StreamReadWriteCloser wraps arbitrary grpc streams around a ReadWriteCloser implementation.
// Users create a new StreamReadWriteCloser by calling NewReadWriteCloser passing a base stream.
// The stream needs to implement RecvMsg and SendMsg (i.e. ClientStream and ServerStream types),
// And a function to set the bytes in the stream message type, get the bytes from the message and close the stream.
// Note that the reader follows the same semantics as <Server|Client>Stream. It is ok to have concurrent writes
// and reads, but it's not ok to have multiple concurrent reads or multiple concurrent writes.
type StreamReadWriteCloser struct {
	io.ReadWriter
	Stream
	StreamMessage

	lastRead []byte
	msgChan  chan []byte
	errChan  chan error
}

// NewReadWriter returns an initialized StreamReadWriteCloser
func NewReadWriteCloser(stream Stream, sm StreamMessage) *StreamReadWriteCloser {
	rw := &StreamReadWriteCloser{
		Stream:        stream,
		StreamMessage: sm,

		// Avoid blocking when Read is never called
		msgChan: make(chan []byte, 1),
		errChan: make(chan error, 1),
	}
	go rw.startReads()
	return rw
}

// startReads reads from the stream in an out of band goroutine in order to handle overflow reads.
func (s *StreamReadWriteCloser) startReads() {
	for {
		msg := s.BuildMsg()
		err := s.Stream.RecvMsg(msg)
		if err != nil {
			s.errChan <- err
			close(s.msgChan)
			return
		}
		b, err := s.GetBytes(msg)
		if err != nil {
			s.errChan <- err
			close(s.msgChan)
			return
		}
		s.msgChan <- b
	}
}

// Read reads from the underlying stream handling cases where the amount read > len(b).
func (s *StreamReadWriteCloser) Read(b []byte) (int, error) {
	if len(s.lastRead) > 0 {
		// we have leftover bytes from last read
		n := copy(b, s.lastRead)
		s.lastRead = s.lastRead[n:]
		return n, nil
	}

	rb, ok := <-s.msgChan
	if !ok {
		err := <-s.errChan
		return 0, err
	}
	n := copy(b, rb)
	// Keep track of any overflow bytes we didn't read
	s.lastRead = rb[n:]
	return n, nil
}

// Write writes b to the underlying stream
func (s *StreamReadWriteCloser) Write(b []byte) (int, error) {
	msg := s.BuildMsg()
	s.SetBytes(msg, b)
	if err := s.Stream.SendMsg(msg); err != nil {
		return 0, err
	}
	return len(b), nil
}

// Close indicates the other side of the stream that we are closing the connection.
func (s *StreamReadWriteCloser) Close() error {
	msg := s.CloseMsg()
	if err := s.Stream.SendMsg(msg); err != nil {
		return err
	}
	return nil
}
