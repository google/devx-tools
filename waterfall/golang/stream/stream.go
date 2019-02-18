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

// Package stream provides functions to operate with data streams.
package stream

import "errors"

var (
	errClosedRead  = errors.New("closed for reading")
	errClosedWrite = errors.New("closed for writing")
)

// Stream defines an interface to send and receive messages.
type Stream interface {
	SendMsg(m interface{}) error
	RecvMsg(m interface{}) error
}

// Given that embedding is not an option, we need to duplicate method names in the
// interface declarations. See https://github.com/golang/go/issues/6977).

// StreamMessageReadWriteCloser defines a generic interface to build messages with byte contents.
type StreamMessageReadWriteCloser interface {
	BuildMsg() interface{}
	GetBytes(interface{}) ([]byte, error)
	SetBytes(interface{}, []byte)
	CloseMsg() interface{}
}

// StreamMessageReader defines a generic interface to read bytes from a message.
type StreamMessageReader interface {
	BuildMsg() interface{}
	GetBytes(interface{}) ([]byte, error)
}

// StreamMessageWriter defines a generic interface to set bytes in a message.
type StreamMessageWriter interface {
	BuildMsg() interface{}
	SetBytes(interface{}, []byte)
}

// StreamMessageCloser defines a generic interface create a message that closes a stream.
type StreamMessageCloser interface {
	BuildMsg() interface{}
	CloseMsg() interface{}
}

// NewReader creates and initializes a new StreamReader.
func NewReader(stream Stream, sm StreamMessageReader) *StreamReader {
	sr := &StreamReader{
		Stream:              stream,
		StreamMessageReader: sm,
		// Avoid blocking when Read is never called
		msgChan: make(chan []byte, 256),
		errChan: make(chan error, 1),
	}
	go sr.startReads()
	return sr
}

// startReads reads from the stream in an out of band goroutine in order to handle overflow reads.
func (s *StreamReader) startReads() {
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

// StreamReader wraps arbitrary grpc streams around a Reader implementation.
type StreamReader struct {
	Stream
	StreamMessageReader
	lastRead []byte
	msgChan  chan []byte
	errChan  chan error
}

// Read reads from the underlying stream handling cases where the amount read > len(b).
func (s *StreamReader) Read(b []byte) (int, error) {
	if len(s.lastRead) > 0 {
		// we have leftover bytes from last read
		n := copy(b, s.lastRead)
		s.lastRead = s.lastRead[n:]
		return n, nil
	}

	// Try to drain the msg channel before returning in order to fulfill the requested slice.
	nt := 0
	for {
		select {
		case rb, ok := <-s.msgChan:
			if !ok {
				if nt == 0 {
					// The channel was closed and nothing was read
					return 0, <-s.errChan
				}
				// Return what we read and return the error on the next read
				return nt, nil
			}
			n := copy(b[nt:], rb)
			nt += n
			s.lastRead = rb[n:]
			if nt == len(b) {
				return nt, nil
			}
		default:
			return nt, nil
		}
	}
}

// NewWriter creates a new StreamWriter.
func NewWriter(stream Stream, sm StreamMessageWriter) *StreamWriter {
	return &StreamWriter{Stream: stream, StreamMessageWriter: sm}
}

// StreamWriter implements a Writer backed by a stream.
type StreamWriter struct {
	Stream
	StreamMessageWriter
}

// StreamWriter writes b to the message in the underlying stream.
func (s *StreamWriter) Write(b []byte) (int, error) {
	msg := s.BuildMsg()
	s.SetBytes(msg, b)
	if err := s.Stream.SendMsg(msg); err != nil {
		return 0, err
	}
	return len(b), nil
}

// StreamReadWriteCloser wraps arbitrary grpc streams around a ReadWriteCloser implementation.
// Users create a new StreamReadWriteCloser by calling NewReadWriteCloser passing a base stream.
// The stream needs to implement RecvMsg and SendMsg (i.e. ClientStream and ServerStream types),
// And a function to set bytes in the stream message type, get bytes from the message and close the stream.
// NOTE: The reader follows the same semantics as <Server|Client>Stream. It is ok to have concurrent writes
// and reads, but it's not ok to have multiple concurrent reads or multiple concurrent writes.
type StreamReadWriteCloser struct {
	Stream
	StreamMessageReadWriteCloser
	r  *StreamReader
	w  *StreamWriter
	cr bool
	cw bool
}

// NewReadWriteCloser returns an initialized StreamReadWriteCloser.
func NewReadWriteCloser(stream Stream, sm StreamMessageReadWriteCloser) *StreamReadWriteCloser {
	rwc := &StreamReadWriteCloser{
		Stream: stream,
		StreamMessageReadWriteCloser: sm,
		r: NewReader(stream, sm),
		w: NewWriter(stream, sm),
	}
	return rwc
}

// Read calls the Read method of the underlying StreamReader if the stream is not closed.
func (s *StreamReadWriteCloser) Read(b []byte) (int, error) {
	if s.cr {
		return 0, errClosedRead
	}
	return s.r.Read(b)
}

// Write writes b to the underlying stream.
func (s *StreamReadWriteCloser) Write(b []byte) (int, error) {
	if s.cw {
		return 0, errClosedWrite
	}
	return s.w.Write(b)
}

// Close closes the the stream.
func (s *StreamReadWriteCloser) Close() error {
	s.cr = true
	s.cw = true
	s.CloseRead()
	return s.CloseWrite()
}

// CloseRead closes the read side of the stream.
func (s *StreamReadWriteCloser) CloseRead() error {
	s.cr = true
	return nil
}

// CloseWrite closes the write side of the stream and signals the other side.
func (s *StreamReadWriteCloser) CloseWrite() error {
	s.cw = true
	return s.Stream.SendMsg(s.CloseMsg())
}
