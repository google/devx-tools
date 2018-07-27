// package forward provides functions to forward connections.
package forward

import (
	"io"
	"sync"
)

// HalfWriteCloser implements a writer that can be closed
type HalfWriteCloser interface {
	io.Writer
	CloseWrite() error
}

// HalfReadCloser implements reader that can be closed
type HalfReadCloser interface {
	io.Reader
	CloseRead() error
}

// HalfReadWriteCloser implements a ReadWriteCloser than can close the read and write side independently.
type HalfReadWriteCloser interface {
	io.Closer
	HalfReadCloser
	HalfWriteCloser
}

// Forward forwards the connection
func Forward(x HalfReadWriteCloser, y HalfReadWriteCloser) error {
	defer x.Close()
	defer y.Close()
	wg := &sync.WaitGroup{}

	// wait for the outgoing and incoming copy goroutines
	wg.Add(2)

	// allow both the outgoing and incoming goroutine to send to the channel
	errCh := make(chan error, 2)

	asyncCopy(x, y, wg, errCh)
	asyncCopy(y, x, wg, errCh)

	doneCh := make(chan struct{})
	go func() {
		wg.Wait()
		close(doneCh)
	}()

	select {
	case <-doneCh:
		return nil
	case err := <-errCh: // just ignore, there's not much we can do.
		return err
	}
}

func asyncCopy(w HalfWriteCloser, r HalfReadCloser, wg *sync.WaitGroup, ch chan error) {
	go func() {
		defer func() {
			r.CloseRead()
			w.CloseWrite()
		}()

		_, err := io.Copy(w, r)
		if err != nil {
			ch <- err
			return
		}
		wg.Done()
	}()
}
