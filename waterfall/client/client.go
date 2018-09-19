// Package client is the reference client implementation for the watefall service
package client

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/waterfall"
	waterfall_grpc "github.com/waterfall/proto/waterfall_go_grpc"
	"golang.org/x/sync/errgroup"
)

// Echo streams back the contents of the request. Useful for testing the connection.
func Echo(ctx context.Context, client waterfall_grpc.WaterfallClient, r []byte) ([]byte, error) {
	stream, err := client.Echo(ctx)
	if err != nil {
		return nil, err
	}
	eg := &errgroup.Group{}
	rec := new(bytes.Buffer)
	eg.Go(func() error {
		for {
			in, err := stream.Recv()
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return err
			}
			rec.Write(in.Payload)
		}
	})
	eg.Go(func() error {
		send := bytes.NewBuffer(r)
		b := make([]byte, 32*1024)
		for {
			n, err := send.Read(b)
			if n > 0 {
				p := &waterfall_grpc.Message{Payload: b[0:n]}
				if err := stream.Send(p); err != nil {
					return err
				}
			}
			if err == io.EOF {
				break
			}
			if err != nil {
				return err
			}
		}
		return stream.CloseSend()
	})
	if err := eg.Wait(); err != nil {
		return nil, err
	}
	return rec.Bytes(), nil
}

// Push pushes a tar stream to the server running in the device.
func Push(ctx context.Context, client waterfall_grpc.WaterfallClient, src, dst string) error {
	rpc, err := client.Push(ctx)
	if err != nil {
		return err
	}

	r, w := io.Pipe()
	defer r.Close()

	eg := &errgroup.Group{}
	eg.Go(func() error {
		err := waterfall.Tar(w, src)
		w.Close()
		return err
	})

	buff := make([]byte, 64*1024)
	eg.Go(func() error {
		for {
			n, err := r.Read(buff)
			if err != nil && err != io.EOF {
				return err
			}

			if n > 0 {
				xfer := &waterfall_grpc.Transfer{Path: dst, Payload: buff[0:n]}
				if err := rpc.Send(xfer); err != nil {
					return err
				}
			}

			if err == io.EOF {
				r, err := rpc.CloseAndRecv()
				if err != nil {
					return err
				}

				if !r.Success {
					return fmt.Errorf(string(r.Err))
				}
				return nil
			}
		}
	})
	return eg.Wait()
}

// Pull request a file/directory from the device and unpacks the contents into the desired path.
func Pull(ctx context.Context, client waterfall_grpc.WaterfallClient, src, dst string) error {
	if _, err := os.Stat(filepath.Dir(dst)); err != nil {
		return err
	}

	xstream, err := client.Pull(ctx, &waterfall_grpc.Transfer{Path: src})
	if err != nil {
		return err
	}

	r, w := io.Pipe()
	eg := &errgroup.Group{}
	eg.Go(func() error {
		err := waterfall.Untar(r, dst)
		r.Close()
		return err
	})

	eg.Go(func() error {
		defer w.Close()
		for {
			fgmt, err := xstream.Recv()
			if err != nil {
				w.Close()
				if err == io.EOF {
					return nil
				}
				return err
			}
			if _, err := w.Write(fgmt.Payload); err != nil {
				return err
			}
		}
	})
	return eg.Wait()
}

// ExecError contains the status code from the executed command.
type ExecError struct {
	ExitCode uint32
}

// Error returns the string representation for an ExecError.
func (e ExecError) Error() string {
	return fmt.Sprintf("non-zero exit code: %d", e.ExitCode)
}

type execMessageWriter struct{}

// BuildMsg returns a reference to a new CmdProgress struct.
func (em execMessageWriter) BuildMsg() interface{} {
	return new(waterfall_grpc.CmdProgress)
}

// SetBytes writes the payload b to stdin.
func (em execMessageWriter) SetBytes(m interface{}, b []byte) {
	msg, ok := m.(*waterfall_grpc.CmdProgress)
	if !ok {
		// this never happens
		panic("incorrect type")
	}
	nb := make([]byte, len(b))
	copy(nb, b)
	msg.Stdin = nb
}

// Exec executes the requested command on the device. Semantics are the same as execve.
func Exec(ctx context.Context, client waterfall_grpc.WaterfallClient, stdout, stderr io.Writer, stdin io.Reader, cmd string, args ...string) error {
	xstream, err := client.Exec(ctx)
	if err != nil {
		return err
	}

	// initializes the command execution on the server
	if err := xstream.Send(
		&waterfall_grpc.CmdProgress{
			Cmd: &waterfall_grpc.Cmd{Path: cmd,
				Args:   args,
				PipeIn: stdin != nil}}); err != nil {
		return err
	}

	eg := &errgroup.Group{}

	if stdin != nil {
		eg.Go(func() error {
			_, err := io.Copy(waterfall.NewWriter(xstream, execMessageWriter{}), stdin)
			if err == nil || err == io.EOF {
				return xstream.CloseSend()
			}
			return err
		})
	}

	var last *waterfall_grpc.CmdProgress
	for {
		pgrs, err := xstream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		if pgrs.Stdout != nil {
			if _, err := stdout.Write(pgrs.Stdout); err != nil {
				return err
			}
		}
		if pgrs.Stderr != nil {
			if _, err := stdout.Write(pgrs.Stderr); err != nil {
				return err
			}
		}
		last = pgrs
	}

	if err := eg.Wait(); err != nil {
		return err
	}

	if last.ExitCode != 0 {
		return ExecError{ExitCode: last.ExitCode}
	}
	return nil
}
