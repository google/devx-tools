package mux

import (
	"io"

	waterfall_grpc_pb "github.com/google/waterfall/proto/waterfall_go_grpc"
)

// Message implements stream.MessageReadWriteCloser interface
type Message struct{}

// BuildMsg returns a new message that can be sent through a forwarding stream.
func (sm Message) BuildMsg() interface{} {
	return new(waterfall_grpc_pb.Message)
}

// GetBytes reads the bytes from the message.
func (sm Message) GetBytes(m interface{}) ([]byte, error) {
	msg, ok := m.(*waterfall_grpc_pb.Message)
	if !ok {
		// this never happens
		panic("incorrect type")
	}

	if len(msg.Payload) == 0 {
		return nil, io.EOF
	}
	return msg.Payload, nil
}

// SetBytes sets the meessage bytes.
func (sm Message) SetBytes(m interface{}, b []byte) {
	msg, ok := m.(*waterfall_grpc_pb.Message)
	if !ok {
		// this never happens
		panic("incorrect type")
	}

	msg.Payload = b
}

// CloseMsg returns a new message to notify the other side that the stream is closed.
func (sm Message) CloseMsg() interface{} {
	return &waterfall_grpc_pb.Message{}
}
