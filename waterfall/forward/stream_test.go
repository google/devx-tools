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

package forward

import (
	"hash/fnv"
	"io"
	"math/rand"
	"net"
	"testing"

	waterfall_grpc "github.com/waterfall/proto/waterfall_go_grpc"
)

const numMsgs = 250000

// chanStream simulates a grpc stream.
// msgIn and msgOut are used by the test to feed and extract the forwarded messages.
type chanStream struct {
	msgIn  chan *waterfall_grpc.ForwardMessage
	msgOut chan *waterfall_grpc.ForwardMessage
}

// SendMsg writes the msg to the stream channel.
func (b *chanStream) SendMsg(m interface{}) error {
	msg := m.(*waterfall_grpc.ForwardMessage)

	bs := make([]byte, len(msg.Payload))
	copy(bs, msg.Payload)
	msg.Payload = bs

	b.msgOut <- msg
	return nil
}

// RecvMsg reads a msg from the stream channel.
func (b *chanStream) RecvMsg(m interface{}) error {
	msg := m.(*waterfall_grpc.ForwardMessage)
	*msg = *<-b.msgIn
	return nil
}

func runEcho(lis net.Listener) error {
	conn, err := lis.Accept()
	if err != nil {
		return err
	}
	defer conn.Close()
	_, err = io.Copy(conn, conn)
	return err
}

// TestStreamForward forwards bytes from a stream to an echo server and verifies bytes xfer == bytes received
func TestStreamForward(t *testing.T) {
	lis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatal(err)
	}
	defer lis.Close()

	eCh := make(chan error, 1)
	go func() {
		runEcho(lis)
	}()

	conn, err := net.Dial(lis.Addr().Network(), lis.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	in := make(chan *waterfall_grpc.ForwardMessage, 1)
	out := make(chan *waterfall_grpc.ForwardMessage, 1)

	sf := NewStreamForwarder(&chanStream{msgIn: in, msgOut: out}, conn.(*net.TCPConn))

	go func() {
		if err := sf.Forward(); err != nil {
			eCh <- err
		}
	}()

	// Hash the bytes as they come to avoid using +512MB of memory.
	sent := fnv.New64a()
	recv := fnv.New64a()
	go func() {
		for i := 0; i < numMsgs; i++ {
			r := make([]byte, 2048)
			// rand always reads full and never returns errors.
			rand.Read(r)
			in <- &waterfall_grpc.ForwardMessage{
				Op:      waterfall_grpc.ForwardMessage_FWD,
				Payload: r}
			// Ok to ignore errors here as weell
			sent.Write(r)
		}

		in <- &waterfall_grpc.ForwardMessage{
			Op: waterfall_grpc.ForwardMessage_CLOSE}
	}()

	total := 0
	dd := make(chan struct{})
	go func() {
		for m := range out {
			if m.Op == waterfall_grpc.ForwardMessage_CLOSE {
				break
			}
			recv.Write(m.Payload)
			total += len(m.Payload)
		}
		close(dd)
	}()

	select {
	case <-dd:
	case err := <-eCh:
		t.Fatalf("Got error: %v\n", err)
	}

	if sent.Sum64() != recv.Sum64() {
		t.Errorf("Sent != Received %d", total)
	}
}
