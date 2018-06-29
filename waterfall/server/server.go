// Package server implements waterfall service
package server

import (
	"context"
	"io"
	"os"
	"os/exec"
	"syscall"

	waterfall_grpc "github.com/waterfall/proto/waterfall_go_grpc"
	"github.com/waterfall/stream"
	"golang.org/x/sync/errgroup"
)

const bufSize = 32 * 1024

// WaterfallServer implements the waterfall gRPC service
type WaterfallServer struct {
	ctx context.Context
}

// Echo exists solely for test purposes. It's a utility function to
// create integration tests between grpc and custom network transports
// like qemu_pipe and usb
func (s *WaterfallServer) Echo(stream waterfall_grpc.Waterfall_EchoServer) error {
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if err := stream.Send(msg); err != nil {
			return err
		}
	}
}

// Push untars a file/dir that it receives over a gRPC stream
func (s *WaterfallServer) Push(rpc waterfall_grpc.Waterfall_PushServer) error {
	xfer, err := rpc.Recv()
	if err != nil {
		return err
	}

	r, w := io.Pipe()
	defer r.Close()
	defer w.Close()

	// Connect the gRPC stream with a reader that untars
	// the contents of the stream into the desired path
	eg, _ := errgroup.WithContext(s.ctx)
	eg.Go(func() error {
		return stream.Untar(r, xfer.Path)
	})

	eg.Go(func() error {
		for {
			if err == io.EOF {
				return rpc.SendAndClose(
					&waterfall_grpc.Transfer{Success: true})
			}
			if err != nil {
				return err
			}
			if _, err := w.Write(xfer.Payload); err != nil {
				return rpc.SendAndClose(
					&waterfall_grpc.Transfer{
						Success: false,
						Err:     []byte(err.Error())})
			}
			xfer, err = rpc.Recv()
		}
	})
	return eg.Wait()
}

// Pull tars up a file/dir and sends it over a gRPC stream
func (s *WaterfallServer) Pull(t *waterfall_grpc.Transfer, ps waterfall_grpc.Waterfall_PullServer) error {
	if _, err := os.Stat(t.Path); os.IsNotExist(err) {
		return err
	}

	r, w := io.Pipe()
	defer r.Close()
	defer w.Close()

	// Connect a writer that traverses a the filesystem
	// tars the contents to the gRPC stream
	eg, _ := errgroup.WithContext(s.ctx)
	eg.Go(func() error {
		defer w.Close()
		return stream.Tar(w, t.Path)
	})

	buff := make([]byte, bufSize)
	eg.Go(func() error {
		defer r.Close()

		for {
			n, err := r.Read(buff)
			if err != nil && err != io.EOF {
				return err
			}

			if n > 0 {
				// No need to populate anything else. Sending the path
				// everytime is just overhead.
				xfer := &waterfall_grpc.Transfer{Payload: buff[0:n]}
				if err := ps.Send(xfer); err != nil {
					return err
				}
			}

			if err == io.EOF {
				return nil
			}
		}
	})
	err := eg.Wait()
	return err
}

const(
	sendOut = iota
	sendErr
)

// execWriter wraps the exec grpc server around the Writer interface
type execWriter struct {
	es waterfall_grpc.Waterfall_ExecServer
	sendTo int32
}

func (ew *execWriter) Write(b []byte) (int, error) {
	cp := &waterfall_grpc.CmdProgress{}
	switch ew.sendTo {
	case sendOut:
		cp.Stdout = b
	case sendErr:
		cp.Stderr = b
	}

	if err := ew.es.Send(cp); err != nil {
		return 0, err
	}
	return len(b), nil
}


// Exec forks a new process with the desired command and pipes its output to the gRPC stream
func (s *WaterfallServer) Exec(cmdMsg *waterfall_grpc.Cmd, es waterfall_grpc.Waterfall_ExecServer) error {
	// Avoid doing any sort of input check, e.g. check that the path exists
	// this way we can forward the exact shell output and exit code.
	cmd := exec.CommandContext(s.ctx, cmdMsg.Path, cmdMsg.Args...)
	cmd.Dir = cmdMsg.Dir
	if cmd.Dir == "" {
		cmd.Dir = "/"
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}
	// Kill the process in case somehting went wrong with the connection
	defer cmd.Process.Kill()
	defer cmd.Process.Release()

	outWriter := &execWriter{es: es, sendTo: sendOut}
	errWriter := &execWriter{es: es, sendTo: sendErr}

	eg, _ := errgroup.WithContext(s.ctx)
	eg.Go(func () error {
		_, err := io.Copy(outWriter, stdout)
		return err

	})
	eg.Go(func () error {
		_, err := io.Copy(errWriter, stderr)
		return err
	})

	if err := eg.Wait(); err != nil {
		return err
	}

	if err := cmd.Wait(); err != nil {
		if exiterr, ok := err.(*exec.ExitError); ok {
			if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
				if err := es.Send(&waterfall_grpc.CmdProgress{
					ExitCode: uint32(status.ExitStatus())}); err != nil {
					return err
				}
				return nil
			}
			// the server always runs on android so the type assertion will aways succeed
			panic("this never happens")
		}
		// we got some other kind of error
		return err
	}
	return es.Send(&waterfall_grpc.CmdProgress{ExitCode: 0})
}

// NewWaterfallServer returns a gRPC server that implements the waterfall service
func NewWaterfallServer(ctx context.Context) *WaterfallServer {
	return &WaterfallServer{ctx: ctx}
}
