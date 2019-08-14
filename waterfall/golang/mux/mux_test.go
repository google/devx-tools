package mux

import (
	"context"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"testing"
	"time"
)

func uniqueSocket() string {
	return fmt.Sprintf("@muxtest_%05d", rand.Int31n(1<<16))
}

func TestConnect(t *testing.T) {
	s := uniqueSocket()
	l, err := net.Listen("unix", s)
	if err != nil {
		t.Fatal(err)
	}

	cchan := make(chan net.Conn, 1)
	echan := make(chan error, 1)

	go func() {
		conn, err := l.Accept()
		cchan <- conn
		echan <- err
	}()

	conn, err := net.Dial("unix", s)
	if err != nil {
		t.Fatal(err)
	}

	if err := <-echan; err != nil {
		t.Fatal(err)
	}

	ml := NewListener(<-cchan)

	cb, err := NewConnBuilder(context.Background(), conn)
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		c, err := ml.Accept()
		if err != nil {
			panic(err)
		}
		log.Println("Accepted connection ...")
		b := make([]byte, 5)
		n, err := io.ReadFull(c, b)
		if err != nil {
			panic(err)
		}
		log.Printf("Read %d %s\n", n, string(b))
	}()

	go func() {
		c, err := cb.Accept()
		if err != nil {
			panic(err)
		}
		log.Println("Created connection ...")
		b := []byte("hello")
		if _, err := c.Write(b); err != nil {
			panic(err)
		}
		log.Printf("Wrote %s\n", string(b))
	}()

	time.Sleep(time.Second * 3)
}
