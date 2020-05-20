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

// Package server implements waterfall service
package server

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	empty_pb "github.com/golang/protobuf/ptypes/empty"
	"github.com/google/waterfall/golang/constants"
	"github.com/google/waterfall/golang/forward"
	"github.com/google/waterfall/golang/stream"
	waterfall_grpc_pb "github.com/google/waterfall/proto/waterfall_go_grpc"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	tmpDir  = "/data/local/tmp"
	sdkProp = "ro.build.version.sdk"
	getProp = "/system/bin/getprop"

	pmCmd          = "pm"
	cmdCmd         = "cmd"
	packageService = "package"

	install        = "install"
	installCreate  = "install-create"
	installWrite   = "install-write"
	installCommit  = "install-commit"
	installAbandon = "install-abandon"
)

// WaterfallServer implements the waterfall gRPC service
type WaterfallServer struct {
	reverseForwardSessionsMutex *sync.Mutex

	reverseForwardSessions map[string]*reverseForwardSession
}

type reverseForwardSession struct {
	addr   string
	connCh chan net.Conn
	lis    net.Listener
}

// Echo exists solely for test purposes. It's a utility function to
// create integration tests between grpc and custom network transports
// like qemu_pipe and usb
func (s *WaterfallServer) Echo(stream waterfall_grpc_pb.Waterfall_EchoServer) error {
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
func (s *WaterfallServer) Push(rpc waterfall_grpc_pb.Waterfall_PushServer) error {
	xfer, err := rpc.Recv()
	if err != nil {
		return err
	}

	r, w := io.Pipe()

	// Connect the gRPC stream with a reader that untars
	// the contents of the stream into the desired path
	eg := &errgroup.Group{}
	eg.Go(func() error {
		defer r.Close()
		return stream.Untar(r, xfer.Path)
	})

	eg.Go(func() error {
		defer w.Close()
		for {
			if err == io.EOF {
				return rpc.SendAndClose(
					&waterfall_grpc_pb.Transfer{Success: true})
			}
			if err != nil {
				return err
			}
			if _, err := w.Write(xfer.Payload); err != nil {
				return rpc.SendAndClose(
					&waterfall_grpc_pb.Transfer{
						Success: false,
						Err:     []byte(err.Error())})
			}
			xfer, err = rpc.Recv()
		}
	})
	return eg.Wait()
}

// Pull tars up a file/dir and sends it over a gRPC stream
func (s *WaterfallServer) Pull(t *waterfall_grpc_pb.Transfer, rpc waterfall_grpc_pb.Waterfall_PullServer) error {
	if _, err := os.Stat(t.Path); os.IsNotExist(err) {
		return err
	}

	r, w := io.Pipe()

	// Connect a writer that traverses a the filesystem
	// tars the contents to the gRPC stream
	eg := &errgroup.Group{}
	eg.Go(func() error {
		defer w.Close()
		return stream.Tar(w, t.Path)
	})

	buff := make([]byte, constants.WriteBufferSize)
	eg.Go(func() error {
		defer r.Close()

		for {
			// try to read full payloads to avoid unnecessary rpc message overhead.
			n, err := io.ReadFull(r, buff)
			if err == io.ErrUnexpectedEOF {
				err = io.EOF
			}

			if err != nil && err != io.EOF {
				return err
			}

			if n > 0 {
				// No need to populate anything else. Sending the path
				// everytime is just overhead.
				xfer := &waterfall_grpc_pb.Transfer{Payload: buff[0:n]}
				if err := rpc.Send(xfer); err != nil {
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

// chanWriter implements Writer and allows us to use the channel for Copy calls
type chanWriter chan []byte

func (c chanWriter) Write(b []byte) (int, error) {
	// make a copy of the slice, the caller might reuse the slice
	// before we have a chance to flush the bytes to the stream.
	nb := make([]byte, len(b))
	copy(nb, b)
	c <- nb
	return len(nb), nil
}

type execMessageReader struct{}

func (em execMessageReader) BuildMsg() interface{} {
	return new(waterfall_grpc_pb.CmdProgress)
}

// SetBytes sets the meessage bytes.
func (em execMessageReader) GetBytes(m interface{}) ([]byte, error) {
	msg, ok := m.(*waterfall_grpc_pb.CmdProgress)
	if !ok {
		// this never happens
		panic("incorrect type")
	}

	return msg.Stdin, nil
}

func exitCode(err error) (int, error) {
	if err == nil {
		return 0, nil
	}
	if errExit, ok := err.(*exec.ExitError); ok {
		return errExit.ProcessState.ExitCode(), nil
	}
	return 0, err
}

// Exec forks a new process with the desired command and pipes its output to the gRPC stream
func (s *WaterfallServer) Exec(rpc waterfall_grpc_pb.Waterfall_ExecServer) error {

	// The first message contains the actual command to execute.
	// Implmented as a streaming method in order to support stdin redirection in the future.
	cmdMsg, err := rpc.Recv()
	if err != nil {
		return err
	}

	// Avoid doing any sort of input check, e.g. check that the path exists
	// this way we can forward the exact shell output and exit code.
	cmd := exec.CommandContext(rpc.Context(), cmdMsg.Cmd.Path, cmdMsg.Cmd.Args...)
	cmd.Dir = cmdMsg.Cmd.Dir
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

	if cmdMsg.Cmd.PipeIn {
		cmd.Stdin = stream.NewReader(rpc, execMessageReader{})
	}

	if err := cmd.Start(); err != nil {
		return err
	}
	// Kill the process in case something went wrong with the connection
	defer cmd.Process.Kill()
	defer cmd.Process.Release()

	// We want to read concurrently from stdout/stderr
	// but we only have one stream to send it to so we need to merge streams before writing.
	// Note that order is not necessarily preserved.
	stdoutCh := make(chan []byte, 1)
	stderrCh := make(chan []byte, 1)

	eg := &errgroup.Group{}

	// Serialize and multiplex stdout/stderr channels to the grpc stream
	eg.Go(func() error {
		_, err := io.Copy(chanWriter(stdoutCh), stdout)
		close(stdoutCh)
		return err

	})
	eg.Go(func() error {
		_, err := io.Copy(chanWriter(stderrCh), stderr)
		close(stderrCh)
		return err
	})
	eg.Go(func() error {
		var o []byte
		var e []byte
		oo := true
		eo := true
		for oo || eo {
			var msg *waterfall_grpc_pb.CmdProgress
			select {
			case o, oo = <-stdoutCh:
				if !oo {
					break
				}
				msg = &waterfall_grpc_pb.CmdProgress{Stdout: o}
			case e, eo = <-stderrCh:
				if !eo {
					break
				}
				msg = &waterfall_grpc_pb.CmdProgress{Stderr: e}
			}
			if msg != nil {
				if err := rpc.Send(msg); err != nil {
					return err
				}
			}
		}
		return nil
	})

	if err := eg.Wait(); err != nil {
		return err
	}

	ec, err := exitCode(cmd.Wait())
	if err != nil {
		return err
	}
	return rpc.Send(&waterfall_grpc_pb.CmdProgress{ExitCode: uint32(ec)})
}

type installReader struct{}

func (r installReader) BuildMsg() interface{} {
	return new(waterfall_grpc_pb.InstallRequest)
}

// SetBytes sets the meessage bytes.
func (r installReader) GetBytes(m interface{}) ([]byte, error) {
	msg, ok := m.(*waterfall_grpc_pb.InstallRequest)
	if !ok {
		// this never happens
		panic("incorrect type")
	}
	return msg.Payload, nil
}

func shell(ctx context.Context, args []string) *exec.Cmd {
	return exec.CommandContext(ctx, "/system/bin/sh", "-c", strings.Join(args, " "))
}

func getVersion() (int, error) {
	cmd := exec.Command(getProp, sdkProp)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.Trim(string(out), "\r\n"))
}

// Install installs the specified package on the device.
func (s *WaterfallServer) Install(rpc waterfall_grpc_pb.Waterfall_InstallServer) error {
	ctx := rpc.Context()

	ins, err := rpc.Recv()
	if err != nil {
		return err
	}

	// Ignore the error, well just default to legacy install
	api, _ := getVersion()

	// 3 Possible cases:
	// Streaming installations are not available (api < 21): use pm install with temp file
	// Streaming installations are available with pm (21 <= api < 24): use pm streamed install
	// Streaming installations are available with cmd (api >= 24): use cmd streamed install
	streamed := false
	useCmd := false
	if api >= 21 {
		streamed = true
	}
	if api >= 25 {
		useCmd = true
	}

	payload := stream.NewReader(rpc, installReader{})
	if !streamed {
		f, err := ioutil.TempFile(tmpDir, "*.apk")
		if err != nil {
			return err
		}
		defer os.Remove(f.Name())
		defer f.Close()

		if _, err := io.Copy(f, payload); err != nil {
			return err
		}

		if err := f.Chmod(os.ModePerm); err != nil {
			return err
		}

		cmd := shell(ctx, append([]string{pmCmd, install}, append(ins.Args, f.Name())...))
		o, err := cmd.CombinedOutput()
		s, err := exitCode(err)
		if err != nil {
			return err
		}

		return rpc.SendAndClose(
			&waterfall_grpc_pb.InstallResponse{
				ExitCode: uint32(s),
				Output:   string(o)})
	}

	insCmd := []string{pmCmd}
	if useCmd {
		insCmd = []string{cmdCmd, packageService}
	}

	action := append(insCmd, installCreate)
	cmd := shell(ctx, append(action, ins.Args...))
	o, err := cmd.CombinedOutput()
	ec, err := exitCode(err)
	if err != nil {
		return err
	}

	if ec != 0 {
		return rpc.SendAndClose(
			&waterfall_grpc_pb.InstallResponse{
				ExitCode: uint32(ec),
				Output:   string(o)})
	}

	out := string(o)
	if strings.Index(out, "[") < 0 || strings.Index(out, "]") < 0 {
		return fmt.Errorf("bad install session: %s", out)
	}

	ss := out[strings.Index(out, "[")+1 : strings.Index(out, "]")]

	action = append(insCmd, installWrite)
	cmd = shell(ctx, append(action, "-S", fmt.Sprintf("%d", ins.ApkSize), ss, "-"))
	cmd.Stdin = payload

	o, err = cmd.CombinedOutput()
	ec, err = exitCode(err)
	if err != nil {
		fmt.Printf("write error %v\n", err)
		shell(ctx, append(insCmd, installAbandon, ss)).Run()
		return err
	}

	if ec != 0 {
		// Ignore error we want to propagate first error
		shell(ctx, append(insCmd, installAbandon, ss)).Run()
		return rpc.SendAndClose(
			&waterfall_grpc_pb.InstallResponse{
				ExitCode: uint32(ec),
				Output:   string(o)})
	}

	action = append(insCmd, installCommit)
	cmd = shell(ctx, append(action, ss))

	o, err = cmd.CombinedOutput()
	ec, err = exitCode(err)
	if err != nil {
		fmt.Printf("commit error %v\n", err)
		// Ignore error we want to propagate first error
		shell(ctx, append(insCmd, installAbandon, ss)).Run()
		return err
	}

	if ec != 0 {
		fmt.Printf("commit non zero code\n")
		// Ignore error we want to propagate first error
		shell(ctx, append(insCmd, installAbandon, ss)).Run()
	}

	return rpc.SendAndClose(
		&waterfall_grpc_pb.InstallResponse{
			ExitCode: uint32(ec),
			Output:   string(o)})
}

func networkDescription(kind waterfall_grpc_pb.ForwardMessage_Kind) (string, error) {
	switch kind {
	case waterfall_grpc_pb.ForwardMessage_TCP:
		return "tcp", nil
	case waterfall_grpc_pb.ForwardMessage_UDP:
		return "udp", nil
	case waterfall_grpc_pb.ForwardMessage_UNIX:
		return "unix", nil
	default:
		return "", status.Error(codes.InvalidArgument, "unsupported network type")
	}
}

// Forward forwards the grpc stream to the requested port.
func (s *WaterfallServer) Forward(rpc waterfall_grpc_pb.Waterfall_ForwardServer) error {
	fwd, err := rpc.Recv()
	if err != nil {
		return err
	}

	kind, err := networkDescription(fwd.Kind)
	if err != nil {
		return err
	}

	conn, err := net.Dial(kind, fwd.Addr)
	if err != nil {
		return err
	}
	sf := forward.NewStreamForwarder(rpc, conn.(forward.HalfReadWriteCloser))
	return sf.Forward()
}

// RverseForward starts forwarding through the provieded stream
func (s *WaterfallServer) ReverseForward(stream waterfall_grpc_pb.Waterfall_ReverseForwardServer) error {
	fwd, err := stream.Recv()
	if err != nil {
		return err
	}

	kind, err := networkDescription(fwd.Kind)
	if err != nil {
		return err
	}

	addr := fmt.Sprintf("%s:%s", kind, fwd.Addr)
	ss, ok := s.reverseForwardSessions[addr]
	if !ok {
		return status.Error(codes.NotFound, fmt.Sprintf(
			"forwarding session for %s does not exist", addr))
	}

	conn := <-ss.connCh
	return forward.NewStreamForwarder(stream, conn.(forward.HalfReadWriteCloser)).Forward()
}

// StartReverseForward forwards the connections from the desired socket through the gRPC stream.
// It works in conjunction with ReverseForward rpc as follows:
// 1) It gets a forwarding request and starts listening for connections in the requested socket
// 2) On accepting a new connection, it sends the client a new request to open a new rpc stream.
//    This is because the server can't directly open a connection on the client.
// 3) The client then sends a new ReverseForward rpc which is used to forward the connection.
// Note that this function blocks until the session is terminated or an error occurs.
func (s *WaterfallServer) StartReverseForward(fwd *waterfall_grpc_pb.ForwardMessage, rpc waterfall_grpc_pb.Waterfall_StartReverseForwardServer) error {
	if fwd.Op != waterfall_grpc_pb.ForwardMessage_OPEN {
		return status.Error(codes.InvalidArgument, "unexpected forward kind")
	}

	kind, err := networkDescription(fwd.Kind)
	if err != nil {
		log.Printf("failed to parse network kind %v %v\n", err, fwd)
		return err
	}

	addr := fmt.Sprintf("%s:%s", kind, fwd.Addr)
	log.Printf("Starting reverse fwd for %v", fwd)

	if _, ok := s.reverseForwardSessions[addr]; ok {
		return status.Error(codes.AlreadyExists, "address already being forwarded")
	}

	lis, err := net.Listen(kind, fwd.Addr)
	if err != nil {
		return err
	}
	defer lis.Close()

	s.reverseForwardSessionsMutex.Lock()
	ss := &reverseForwardSession{
		addr:   addr,
		lis:    lis,
		connCh: make(chan net.Conn, 1),
	}
	s.reverseForwardSessions[addr] = ss
	s.reverseForwardSessionsMutex.Unlock()

	for {
		log.Printf("Listening for connections to forward %v ...", fwd)
		conn, err := lis.Accept()
		if err != nil {
			return err
		}

		ss.connCh <- conn
		if err := rpc.Send(
			&waterfall_grpc_pb.ForwardMessage{Op: waterfall_grpc_pb.ForwardMessage_OPEN}); err != nil {
			delete(s.reverseForwardSessions, addr)
			return err
		}
	}
}

// StopReverseForward stops a reverse forwarding session.
func (s *WaterfallServer) StopReverseForward(ctxt context.Context, fwd *waterfall_grpc_pb.ForwardMessage) (*empty_pb.Empty, error) {
	s.reverseForwardSessionsMutex.Lock()
	defer s.reverseForwardSessionsMutex.Unlock()

	kind, err := networkDescription(fwd.Kind)
	if err != nil {
		return nil, err
	}
	addr := fmt.Sprintf("%s:%s", kind, fwd.Addr)

	ss, ok := s.reverseForwardSessions[addr]
	if !ok {
		return &empty_pb.Empty{}, nil
	}
	delete(s.reverseForwardSessions, addr)

	ss.lis.Close()
	close(ss.connCh)

	return &empty_pb.Empty{}, nil
}

// Version returns the version of the server.
func (s *WaterfallServer) Version(context.Context, *empty_pb.Empty) (*waterfall_grpc_pb.VersionMessage, error) {
	return &waterfall_grpc_pb.VersionMessage{Version: "0.0"}, nil
}

// New initializes a new waterfall server
func New() *WaterfallServer {
	return &WaterfallServer{
		reverseForwardSessionsMutex: new(sync.Mutex),
		reverseForwardSessions:      map[string]*reverseForwardSession{},
	}
}
