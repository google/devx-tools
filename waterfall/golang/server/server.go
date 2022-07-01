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
	"time"

	empty_pb "github.com/golang/protobuf/ptypes/empty"
	"github.com/google/waterfall/golang/constants"
	"github.com/google/waterfall/golang/forward"
	"github.com/google/waterfall/golang/snapshot"
	"github.com/google/waterfall/golang/stream"
	waterfall_grpc_pb "github.com/google/waterfall/proto/waterfall_go_grpc"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
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
	noStreaming    = "no-streaming"
)

// WaterfallServer implements the waterfall gRPC service
type WaterfallServer struct {
	reverseForwardSessionsMutex sync.RWMutex

	reverseForwardSessions map[string]*reverseForwardSession
	server                 *grpc.Server
}

type reverseForwardSession struct {
	addr    string
	lis     net.Listener
	mu      sync.Mutex
	connMap map[uint64]net.Conn
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
		err := stream.Untar(r, xfer.Path)
		if err != nil {
			log.Printf("stream Untar failed: %v\n", err)
		}
		defer r.CloseWithError(err)
		return err
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
		// see use a pipe insted of assigning directly to cmd.Stdin
		// see https://github.com/golang/go/issues/7990
		si, err := cmd.StdinPipe()
		if err != nil {
			return err
		}

		go func() {
			defer si.Close()
			io.Copy(si, stream.NewReader(rpc, execMessageReader{}))
		}()
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
		log.Printf("Install Recv failed: %v", err)
		return err
	}
	log.Println("Starting Install")
	// Ignore the error, well just default to legacy install
	api, _ := getVersion()

	var args []string
	forceNoStreaming := false
	for _, a := range ins.Args {
		if strings.Contains(a, noStreaming) {
			forceNoStreaming = true
		} else {
			args = append(args, a)
		}
	}
	// 4 Possible cases:
	// Streaming installations are not available (api < 21) or "--no-streaming" flag provided: use pm install with temp file
	// Streaming installations are available with pm (21 <= api < 24): use pm streamed install
	// Streaming installations are available with cmd (api >= 24): use cmd streamed install
	streamed := false
	useCmd := false
	if api >= 21 && !forceNoStreaming {
		streamed = true
	}
	if api >= 25 {
		useCmd = true
	}

	payload := stream.NewReader(rpc, installReader{})
	if !streamed {
		log.Println("Installing via pm install without streaming")
		f, err := ioutil.TempFile(tmpDir, "*.apk")
		if err != nil {
			log.Printf("Install failed to create temp APK file: %v", err)
			return err
		}
		defer os.Remove(f.Name())
		defer f.Close()

		if _, err := io.Copy(f, payload); err != nil {
			log.Printf("Install failed to copy APK payload to temp file: %v", err)
			return err
		}

		if err := f.Chmod(os.ModePerm); err != nil {
			log.Printf("Install failed to chmod temp APK file: %v", err)
			return err
		}

		cmd := shell(ctx, append([]string{pmCmd, install}, append(args, f.Name())...))
		o, err := cmd.CombinedOutput()
		s, err := exitCode(err)
		if err != nil {
			log.Printf("pm install %s failed %v: %s", f.Name(), err, string(o))
			return err
		}

		return rpc.SendAndClose(
			&waterfall_grpc_pb.InstallResponse{
				ExitCode: uint32(s),
				Output:   string(o)})
	}
	log.Println("Installing via streamed install")
	insCmd := []string{pmCmd}
	if useCmd {
		insCmd = []string{cmdCmd, packageService}
	}

	action := append(insCmd, installCreate)
	cmd := shell(ctx, append(action, args...))
	o, err := cmd.CombinedOutput()
	ec, err := exitCode(err)
	if err != nil {
		log.Printf("failed to create install session %v: %s", err, string(o))
		return err
	}

	if ec != 0 {
		return rpc.SendAndClose(
			&waterfall_grpc_pb.InstallResponse{
				ExitCode: uint32(ec),
				Output:   string(o)})
	}
	log.Println("Created streamed install session")
	out := string(o)
	if strings.Index(out, "[") < 0 || strings.Index(out, "]") < 0 {
		log.Printf("bad install session: %s", out)
		return fmt.Errorf("bad install session: %s", out)
	}

	ss := out[strings.Index(out, "[")+1 : strings.Index(out, "]")]

	action = append(insCmd, installWrite)
	cmd = shell(ctx, append(action, "-S", fmt.Sprintf("%d", ins.ApkSize), ss, "-"))
	cmd.Stdin = payload

	o, err = cmd.CombinedOutput()
	ec, err = exitCode(err)
	if err != nil {
		log.Printf("Install write error %v: %s\n", err, string(o))
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

	log.Println("Committing streamed install session")

	action = append(insCmd, installCommit)
	cmd = shell(ctx, append(action, ss))

	o, err = cmd.CombinedOutput()
	ec, err = exitCode(err)
	if err != nil {
		log.Printf("Install commit error %v: %s\n", err, string(o))
		// Ignore error we want to propagate first error
		shell(ctx, append(insCmd, installAbandon, ss)).Run()
		return err
	}

	if ec != 0 {
		log.Printf("commit non zero code\n")
		// Ignore error we want to propagate first error
		shell(ctx, append(insCmd, installAbandon, ss)).Run()
	} else {
		log.Println("Streamed install session comitted successfully")
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
		log.Printf("failed to recv forwarding stream: %v", err)
		return err
	}

	kind, err := networkDescription(fwd.Kind)
	if err != nil {
		log.Printf("failed to determine network forwarding type: %v", err)
		return err
	}

	conn, err := net.Dial(kind, fwd.Addr)
	if err != nil {
		log.Printf("failed to ")
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
	pts := strings.SplitN(fwd.Addr, "/", 2)
	if len(pts) != 2 {
		log.Printf("Failed to parse reverse forwarding src address %s", fwd.Addr)
		return status.Error(codes.InvalidArgument, fmt.Sprintf("unable to parse src address \"%s\"", fwd.Addr))
	}

	addr := fmt.Sprintf("%s:%s", kind, pts[0])
	s.reverseForwardSessionsMutex.RLock()
	ss, ok := s.reverseForwardSessions[addr]
	s.reverseForwardSessionsMutex.RUnlock()
	cID, err := strconv.ParseUint(pts[1], 10, 64)
	if err != nil {
		log.Printf("unable to parse cID from src address \"%s\"", fwd.Addr)
		return status.Error(codes.InvalidArgument, fmt.Sprintf("unable to parse cID from src address \"%s\"", fwd.Addr))
	}
	if !ok {
		log.Printf("forwarding session for %s does not exist", addr)
		return status.Error(codes.NotFound, fmt.Sprintf(
			"forwarding session for %s does not exist", addr))
	}

	ss.mu.Lock()
	conn, ok := ss.connMap[cID]
	delete(ss.connMap, cID)
	ss.mu.Unlock()
	if !ok {
		log.Printf("forwarding session for %s does not exist", fwd.Addr)
		return status.Error(codes.NotFound, fmt.Sprintf(
			"forwarding session for %s does not exist", fwd.Addr))
	}
	return forward.NewStreamForwarder(stream, conn.(forward.HalfReadWriteCloser)).Forward()
}

func listenerPort(l net.Listener) int {
	switch addr := l.Addr().(type) {
	case *net.TCPAddr:
		return addr.Port
	case *net.UDPAddr:
		return addr.Port
	}
	return 0
}

func buildListener(kind string, address string) (net.Listener, error) {
	// net.Listen("tcp", "localhost:0") relies on the kernel picking the port
	// The chosen port is returned in lis.Addr()
	// However, in some cases the returned port is 0
	// We retry creating the listener in an attempt to workaround this problem.
	for i := 0; i < 3; i++ {
		lis, err := net.Listen(kind, address)
		if err != nil {
			return nil, err
		}
		if listenerPort(lis) != 0 {
			return lis, nil
		} 
		log.Println("failed to bind random port for reverse forwarding")
		lis.Close()
	}
	return nil, fmt.Errorf("failed to bind to free port when any available port requested")
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

	s.reverseForwardSessionsMutex.RLock()
	_, ok := s.reverseForwardSessions[addr]
	s.reverseForwardSessionsMutex.RUnlock()
	if ok {
		log.Printf("Address %s already being forwarded")
		return status.Error(codes.AlreadyExists, "address already being forwarded")
	}

	lis, err := buildListener(kind, fwd.Addr)
	if err != nil {
		log.Printf("Failed to listen on address %s: %v", fwd.Addr, err)
		return err
	}
	defer lis.Close()

	// If the user requested port 0 then the first message sent by the server announces the chosen port
	announceAddr := fwd.Addr == "localhost:0" && (kind == "tcp" || kind == "udp")

	if announceAddr {
		fwd.Addr = fmt.Sprintf("localhost:%d", listenerPort(lis))
		if fwd.Kind == waterfall_grpc_pb.ForwardMessage_TCP {
			addr = fmt.Sprintf("tcp:%s", fwd.Addr)
		}
		if fwd.Kind == waterfall_grpc_pb.ForwardMessage_UDP {
			addr = fmt.Sprintf("udp:%s", fwd.Addr)
		}
	}
	ss := &reverseForwardSession{
		addr:    addr,
		lis:     lis,
		connMap: make(map[uint64]net.Conn),
	}
	s.reverseForwardSessionsMutex.Lock()
	s.reverseForwardSessions[addr] = ss
	s.reverseForwardSessionsMutex.Unlock()

	if announceAddr {
		if err := rpc.Send(
			&waterfall_grpc_pb.ForwardMessage{
				Op:   waterfall_grpc_pb.ForwardMessage_ANNOUNCE,
				Addr: addr,
				Kind: fwd.Kind,
			}); err != nil {
		}
	}
	var cID uint64 = 0

	for {
		log.Printf("Listening for connections to reverse forward %v ...", fwd)
		conn, err := lis.Accept()
		if err != nil {
			log.Printf("Failed to accept reverse forwarding connection for %v", fwd)
			return err
		}

		cID = cID + 1
		ss.mu.Lock()
		ss.connMap[cID] = conn
		ss.mu.Unlock()
		if err := rpc.Send(
			&waterfall_grpc_pb.ForwardMessage{
				Op:   waterfall_grpc_pb.ForwardMessage_OPEN,
				Addr: fmt.Sprintf("%s/%d", fwd.Addr, cID),
				Kind: fwd.Kind,
			}); err != nil {
			log.Printf("Reverse Forward failed to send request to client to forward %s: %v", fwd.Addr, err)
			s.reverseForwardSessionsMutex.Lock()
			delete(s.reverseForwardSessions, addr)
			s.reverseForwardSessionsMutex.Unlock()
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
	ss.mu.Lock()
	for _, c := range ss.connMap {
		c.Close()
	}
	ss.mu.Unlock()

	return &empty_pb.Empty{}, nil
}

// Version returns the version of the server.
func (s *WaterfallServer) Version(context.Context, *empty_pb.Empty) (*waterfall_grpc_pb.VersionMessage, error) {
	return &waterfall_grpc_pb.VersionMessage{Version: "0.0"}, nil
}

func (s *WaterfallServer) SnapshotShutdown(ctx context.Context, _ *empty_pb.Empty) (*empty_pb.Empty, error) {
	go s.server.GracefulStop()
	go func() {
		time.Sleep(5 * time.Second)
		log.Println("Server still running after 5s, force stopping...")
		s.server.Stop()
	}()
	return &empty_pb.Empty{}, snapshot.SetSnapshotProp(ctx)
}

// New initializes a new waterfall server
func New(server *grpc.Server) *WaterfallServer {
	return &WaterfallServer{
		reverseForwardSessions: map[string]*reverseForwardSession{},
		server:                 server,
	}
}
