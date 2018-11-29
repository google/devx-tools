package forward

import (
	"io"

	"github.com/waterfall"
	waterfall_grpc "github.com/waterfall/proto/waterfall_go_grpc"
)

// StreamForwarder provides a mechanism to forward bytes between a stream and a connection.
type StreamForwarder struct {
	stream *waterfall.StreamReadWriteCloser
	conn   HalfReadWriteCloser
}

type forwardMsg struct{}

// BuildMsg returns a new message that can be sent through a forwarding stream.
func (fm forwardMsg) BuildMsg() interface{} {
	return new(waterfall_grpc.ForwardMessage)
}

// GetBytes reads the bytes from the message.
func (fm forwardMsg) GetBytes(m interface{}) ([]byte, error) {
	msg, ok := m.(*waterfall_grpc.ForwardMessage)
	if !ok {
		// this never happens
		panic("incorrect type")
	}
	if msg.Op == waterfall_grpc.ForwardMessage_CLOSE {
		return nil, io.EOF
	}
	return msg.Payload, nil
}

// SetBytes sets the meessage bytes.
func (fm forwardMsg) SetBytes(m interface{}, b []byte) {
	msg, ok := m.(*waterfall_grpc.ForwardMessage)
	if !ok {
		// this never happens
		panic("incorrect type")
	}

	msg.Payload = b
}

// CloseMsg returns a new message to notify the other side that the stream is closed.
func (fm forwardMsg) CloseMsg() interface{} {
	return &waterfall_grpc.ForwardMessage{Op: waterfall_grpc.ForwardMessage_CLOSE}
}

// NewStreamForwarder returns a new stream <-> conn forwarder.
func NewStreamForwarder(s waterfall.Stream, conn HalfReadWriteCloser) *StreamForwarder {
	return &StreamForwarder{stream: waterfall.NewReadWriteCloser(s, forwardMsg{}), conn: conn}
}

// Forward starts forwarding from the stream/conn to the stream/conn.
func (fwdr *StreamForwarder) Forward() error {
	return Forward(fwdr.stream, fwdr.conn)
}

// Stop forcefully closes both ends of the forwarder to stop the forwarding session.
func (fwdr *StreamForwarder) Stop() error {
	fwdr.stream.Close()
	fwdr.conn.Close()
	return nil
}
