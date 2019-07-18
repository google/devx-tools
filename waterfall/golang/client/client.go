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

// Package client is the reference client implementation for the watefall service
package client

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/google/waterfall/golang/stream"
	waterfall_grpc "github.com/google/waterfall/proto/waterfall_go_grpc"
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
	r, w := io.Pipe()
	defer r.Close()

	eg := &errgroup.Group{}
	eg.Go(func() error {
		err := stream.Tar(w, src)
		w.Close()
		return err
	})

	if err := pushTar(ctx, eg, client, r, dst); err != nil {
		return err
	}
	return eg.Wait()
}

func pushTar(ctx context.Context, eg *errgroup.Group, client waterfall_grpc.WaterfallClient, r *io.PipeReader, dst string) error {
	rpc, err := client.Push(ctx)
	if err != nil {
		return err
	}
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
	return nil
}

// PushBytes pushes src reader contents to a dst filepath on device.
func PushBytes(ctx context.Context, client waterfall_grpc.WaterfallClient, srcBytes []byte, dst string) error {
	r, w := io.Pipe()
	defer r.Close()

	eg := &errgroup.Group{}
	eg.Go(func() error {
		err := stream.TarBytes(w, srcBytes)
		w.Close()
		return err
	})

	if err := pushTar(ctx, eg, client, r, dst); err != nil {
		return err
	}
	return eg.Wait()
}

// Pull request a file/directory from the device and unpacks the contents into the desired path.
func Pull(ctx context.Context, client waterfall_grpc.WaterfallClient, src, dst string) error {
	if _, err := os.Stat(filepath.Dir(dst)); err != nil {
		return err
	}

	r, w := io.Pipe()
	eg := &errgroup.Group{}
	if err := pullStream(ctx, client, eg, w, src); err != nil {
		return err
	}
	eg.Go(func() error {
		err := stream.Untar(r, dst)
		r.Close()
		return err
	})
	return eg.Wait()
}

func pullStream(ctx context.Context, client waterfall_grpc.WaterfallClient, eg *errgroup.Group, w *io.PipeWriter, src string) error {
	xstream, err := client.Pull(ctx, &waterfall_grpc.Transfer{Path: src})
	if err != nil {
		return err
	}
	eg.Go(func() error {
		defer w.Close()
		for {
			fgmt, err := xstream.Recv()
			if err != nil {
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
	return nil
}

// PullBytes requests contents of a single src file from the device and provides it on dst writer.
func PullBytes(ctx context.Context, client waterfall_grpc.WaterfallClient, dst io.Writer, src string) error {
	r, w := io.Pipe()
	eg := &errgroup.Group{}
	if err := pullStream(ctx, client, eg, w, src); err != nil {
		return err
	}
	eg.Go(func() error {
		err := stream.UntarBytes(dst, r)
		r.Close()
		return err
	})
	return eg.Wait()
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
func Exec(ctx context.Context, client waterfall_grpc.WaterfallClient, stdout, stderr io.Writer, stdin io.Reader, cmd string, args ...string) (int, error) {
	xstream, err := client.Exec(ctx)
	if err != nil {
		return 0, err
	}

	// initializes the command execution on the server
	if err := xstream.Send(
		&waterfall_grpc.CmdProgress{
			Cmd: &waterfall_grpc.Cmd{Path: cmd,
				Args:   args,
				PipeIn: stdin != nil}}); err != nil {
		return 0, err
	}

	eg := &errgroup.Group{}

	if stdin != nil {
		eg.Go(func() error {
			_, err := io.Copy(stream.NewWriter(xstream, execMessageWriter{}), stdin)
			if err == nil || err == io.EOF {
				return xstream.CloseSend()
			}
			return err
		})
	} else {
		if err = xstream.CloseSend(); err != nil {
			return 0, err
		}
	}

	var last *waterfall_grpc.CmdProgress
	for {
		pgrs, err := xstream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			return 0, err
		}

		if pgrs.Stdout != nil && stdout != nil {
			if _, err := stdout.Write(pgrs.Stdout); err != nil {
				return 0, err
			}
		}
		if pgrs.Stderr != nil && stderr != nil {
			if _, err := stderr.Write(pgrs.Stderr); err != nil {
				return 0, err
			}
		}
		last = pgrs
	}

	if err := eg.Wait(); err != nil {
		return 0, err
	}

	return int(last.ExitCode), nil
}
