package waterfall

import (
	"bytes"
	"io"
	"sync"
	"testing"
)

var (
	testBytes = makeBytes()
	bytesBuff = concat(testBytes)
)

type byteMsg struct {
	bs  []byte
	err error
}

type chanStream struct {
	msgIn  chan *byteMsg
	msgOut chan *byteMsg
}

// SendMsg writes the msg to the stream channel.
func (b *chanStream) SendMsg(m interface{}) error {
	msg := m.(*byteMsg)
	b.msgOut <- msg
	return nil
}

// RecvMsg reads a msg from the stream channel.
func (b *chanStream) RecvMsg(m interface{}) error {
	msg := m.(*byteMsg)
	*msg = *<-b.msgIn
	return nil
}

type streamMsg struct {
}

func (sm streamMsg) BuildMsg() interface{} {
	return &byteMsg{}
}

func (sm streamMsg) GetBytes(m interface{}) ([]byte, error) {
	msg := m.(*byteMsg)
	if msg.err != nil {
		return nil, msg.err
	}
	return msg.bs, nil
}

func (sm streamMsg) SetBytes(m interface{}, bs []byte) {
	msg := m.(*byteMsg)
	msg.bs = bs
}

func (sm streamMsg) CloseMsg() interface{} {
	return &byteMsg{err: io.EOF}
}

func makeBytes() [][]byte {
	// 512 * 512 = 256KB transfer size. Note that during a test we keep around
	// three copies of this (original, bytes sent and bytes received).
	// In order to test larger transfers one would need to modify this.
	size := 512
	bss := make([][]byte, size)
	for i := 0; i < size; i++ {
		bs := make([]byte, size)
		for j := 0; j < size; j++ {
			bs[j] = byte(j % 256)
		}
		bss[i] = bs
	}
	return bss
}

func concat(bss [][]byte) *bytes.Buffer {
	buff := new(bytes.Buffer)
	for _, bs := range bss {
		if _, err := buff.Write(bs); err != nil {
			// Probably out of memory. Not much we can do
			panic(err)
		}
	}
	return buff
}

func TestStream(t *testing.T) {
	a := new(bytes.Buffer)
	b := new(bytes.Buffer)

	inA := make(chan *byteMsg)
	inB := make(chan *byteMsg)

	sa := &chanStream{msgIn: inA, msgOut: inB}
	sb := &chanStream{msgIn: inB, msgOut: inA}

	sideA := NewReadWriteCloser(sa, streamMsg{})
	sideB := NewReadWriteCloser(sb, streamMsg{})

	done := &sync.WaitGroup{}
	done.Add(2)
	errCh := make(chan error, 2)
	doneCh := make(chan struct{})

	go func() {
		for _, bs := range testBytes {
			if _, err := sideA.Write(bs); err != nil {
				errCh <- err
				return
			}
			if _, err := a.Write(bs); err != nil {
				errCh <- err
				return
			}
		}
		// Close write side
		sideA.Close()
		done.Done()
	}()

	go func() {
		if _, err := io.Copy(b, sideB); err != nil {
			errCh <- err
			return
		}
		done.Done()
	}()

	go func() {
		done.Wait()
		close(doneCh)
	}()

	select {
	case <-doneCh:
		if !bytes.Equal(a.Bytes(), b.Bytes()) || !bytes.Equal(a.Bytes(), bytesBuff.Bytes()) {
			t.Errorf("missmatching byte arrays during r/w operations")
		}
	case err := <-errCh:
		t.Errorf("error sending bytes through stream: %v", err)
	}
}

func TestShortReads(t *testing.T) {
	xfer := new(bytes.Buffer)

	inA := make(chan *byteMsg)
	inB := make(chan *byteMsg)

	sa := &chanStream{msgIn: inA, msgOut: inB}
	sb := &chanStream{msgIn: inB, msgOut: inA}

	sideA := NewReadWriteCloser(sa, streamMsg{})
	sideB := NewReadWriteCloser(sb, streamMsg{})

	done := &sync.WaitGroup{}
	done.Add(2)
	errCh := make(chan error, 2)
	doneCh := make(chan struct{})

	go func() {
		for _, bs := range testBytes {
			if _, err := sideA.Write(bs); err != nil {
				errCh <- err
				return
			}
		}
		// Close write side
		sideA.Close()
		done.Done()
	}()

	go func() {
		// Use a small slice to test case where message payload won't fit in read buff.
		br := make([]byte, 128)
		for {
			n, err := sideB.Read(br)
			if err == io.EOF {
				break
			}
			if err != nil {
				errCh <- err
				return
			}
			if _, err := xfer.Write(br[:n]); err != nil {
				errCh <- err
				return
			}
		}
		done.Done()
	}()

	go func() {
		done.Wait()
		close(doneCh)
	}()

	select {
	case <-doneCh:
		if !bytes.Equal(xfer.Bytes(), bytesBuff.Bytes()) {
			t.Errorf("missmatching byte arrays during r/w operations")
		}
	case err := <-errCh:
		t.Errorf("error sending bytes through stream: %v", err)
	}
}
