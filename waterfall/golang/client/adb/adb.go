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

// Package adb provides routines for an adb compatible CLI client.
package adb

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	empty_pb "github.com/golang/protobuf/ptypes/empty"
	"github.com/google/waterfall/golang/client"
	waterfall_grpc "github.com/google/waterfall/proto/waterfall_go_grpc"
	"google.golang.org/grpc"
)

const (
	androidADBEnv          = "ANDROID_ADB"
	androidSDKEnv          = "ANDROID_SDK_HOME"
	propagateStatusCodeEnv = "WATERFALL_PROPAGATE_STATUS_CODE"
)

// ParseError represents an command line parsing error.
type ParseError struct{}

// Error returns the empty string. We use this to fallback to regular ADB.
// This is only used to do a type assertion on the error.
func (e ParseError) Error() string {
	return ""
}

// ClientFn allows the user to custumize the way the client connection is established.
type ClientFn func() (*grpc.ClientConn, error)

// ParsedArgs args represents a parsed command line.
type ParsedArgs struct {
	Device  string
	Command string
	Args    []string
}

type cmdFn func(context.Context, ClientFn, []string) error

var (
	// Commands maps from a command name to the function to execute the command.
	Commands = map[string]cmdFn{
		"shell":     shellFn,
		"push":      pushFn,
		"pull":      pullFn,
		"forward":   forwardFn,
		"bugreport": bugreportFn,
		"logcat":    passthroughFn,
		"install":   installFn,
		"uninstall": uninstallFn,
	}
)

func propagateStatusCode(s int) {
	p := os.Getenv(propagateStatusCodeEnv)
	if p == "" {
		return
	}

	b, err := strconv.ParseBool(p)
	if err != nil {
		return
	}

	if b && s != 0 {
		os.Exit(s)
	}
}

func platformADB() (string, error) {
	p := os.Getenv(androidADBEnv)
	if p != "" {
		return p, nil
	}

	p = os.Getenv(androidSDKEnv)
	if p != "" {
		return filepath.Join(p, "platform-tools/adb"), nil
	}
	return "", errors.New("unable to find platform ADB, neither ANDROID_ADB or ANDROID_SDK_HOME set")
}

// Fallback forks a new adb process and executes the provided command.
// Note that this method is terminal. We transfer control to adb.
// It will either fatal out or call Exit.
func Fallback(args []string) {
	adbBin, err := platformADB()
	if err != nil {
		log.Fatal(err)
	}

	cmd := exec.Command(adbBin, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = procAttrs

	if err := cmd.Start(); err != nil {
		log.Fatal(err)
	}

	if err := cmd.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		log.Fatal(err)
	}
	os.Exit(0)
}

func exeStdout(ctx context.Context, c waterfall_grpc.WaterfallClient, cmd string, args ...string) (int, error) {
	return client.Exec(ctx, c, os.Stdout, os.Stdout, nil, cmd, args...)
}

func exeString(ctx context.Context, c waterfall_grpc.WaterfallClient, in io.Reader, cmd string, args ...string) (int, string, error) {
	b := bytes.NewBuffer([]byte{})
	s, err := client.Exec(ctx, c, b, b, in, cmd, args...)
	return s, b.String(), err
}

func shellStdout(ctx context.Context, c waterfall_grpc.WaterfallClient, cmd string, args ...string) error {
	_, err := shellIO(ctx, c, os.Stdout, os.Stdout, nil, cmd, args...)
	return err
}

func shellString(ctx context.Context, c waterfall_grpc.WaterfallClient, in io.Reader, cmd string, args ...string) (int, string, error) {
	b := bytes.NewBuffer([]byte{})
	s, err := shellIO(ctx, c, b, b, in, cmd, args...)
	return s, b.String(), err
}

func shellIO(ctx context.Context, c waterfall_grpc.WaterfallClient, stdout io.Writer, stderr io.Writer, stdin io.Reader, cmd string, args ...string) (int, error) {
	s, err := client.Exec(ctx, c, stdout, stderr, stdin, "/system/bin/sh", "-c", fmt.Sprintf("%s %s", cmd, strings.Join(args, " ")))
	propagateStatusCode(s)
	return s, err
}

func shellFn(ctx context.Context, cfn ClientFn, args []string) error {
	if len(args) == 1 {
		// Not an actual error, but user is requesting an interactive shell session.
		// Return this in order to fallback to adb.
		return ParseError{}
	}
	conn, err := cfn()
	if err != nil {
		return err
	}
	defer conn.Close()
	return shellStdout(ctx, waterfall_grpc.NewWaterfallClient(conn), args[1], args[2:]...)
}

func pushFn(ctx context.Context, cfn ClientFn, args []string) error {
	if len(args) != 3 {
		return ParseError{}
	}

	conn, err := cfn()
	if err != nil {
		return err
	}
	defer conn.Close()

	return client.Push(ctx, waterfall_grpc.NewWaterfallClient(conn), args[1], args[2])
}

func pullFn(ctx context.Context, cfn ClientFn, args []string) error {
	if len(args) != 3 {
		return ParseError{}
	}

	conn, err := cfn()
	if err != nil {
		return err
	}
	defer conn.Close()

	return client.Pull(ctx, waterfall_grpc.NewWaterfallClient(conn), args[1], args[2])
}

func installFn(ctx context.Context, cfn ClientFn, args []string) error {
	if len(args) < 2 {
		return ParseError{}
	}

	path := args[len(args)-1]
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	conn, err := cfn()
	if err != nil {
		return err
	}
	defer conn.Close()

	c := waterfall_grpc.NewWaterfallClient(conn)
	r, err := client.Install(ctx, c, f, args[1:len(args)-1]...)

	// Log output of install command as is
	fmt.Printf(r)
	return err
}

func parseFwd(addr string, reverse bool) (string, error) {
	if reverse {
		if strings.HasPrefix(addr, "tcp:") {
			pts := strings.SplitN(addr, ":", 3)
			return "tcp:" + pts[2], nil
		}
		if strings.HasPrefix(addr, "unix:@") {
			return "localabstract:" + addr[6:], nil
		}
		if strings.HasPrefix(addr, "unix:") {
			return "localreserved:" + addr[5:], nil
		}
		return "", ParseError{}
	}

	pts := strings.Split(addr, ":")
	if len(pts) != 2 {
		return "", ParseError{}
	}

	switch pts[0] {
	case "tcp":
		return "tcp:localhost:" + pts[1], nil
	case "localabstract":
		return "unix:@" + pts[1], nil
	case "localreserved":
		fallthrough
	case "localfilesystem":
		return "unix:" + pts[1], nil
	default:
		return "", ParseError{}
	}
}

func forwardFn(ctx context.Context, cfn ClientFn, args []string) error {
	if len(args) < 2 || len(args) > 4 {
		return ParseError{}
	}

	conn, err := cfn()
	if err != nil {
		return err
	}
	defer conn.Close()
	c := waterfall_grpc.NewPortForwarderClient(conn)

	var src, dst string
	switch args[1] {
	case "--list":
		ss, err := c.List(ctx, &empty_pb.Empty{})
		if err != nil {
			return err
		}
		for _, s := range ss.Sessions {
			src, err := parseFwd(s.Src, true)
			if err != nil {
				return err
			}

			dst, err := parseFwd(s.Dst, true)
			if err != nil {
				return err
			}
			fmt.Printf("localhost:foo %s %s\n", src, dst)
		}
		return nil
	case "--remove":
		if len(args) != 3 {
			return ParseError{}
		}
		fwd, err := parseFwd(args[2], false)
		if err != nil {
			return err
		}
		_, err = c.Stop(ctx, &waterfall_grpc.PortForwardRequest{
			Session: &waterfall_grpc.ForwardSession{Src: fwd}})
		return err
	case "--remove-all":
		if len(args) != 2 {
			return ParseError{}
		}
		_, err = c.StopAll(ctx, &empty_pb.Empty{})
		return err
	case "--no-rebind":
		if src, err = parseFwd(args[2], false); err != nil {
			return err
		}
		if dst, err = parseFwd(args[3], false); err != nil {
			return err
		}
		_, err = c.ForwardPort(ctx, &waterfall_grpc.PortForwardRequest{
			Session: &waterfall_grpc.ForwardSession{Src: src, Dst: dst}, Rebind: false})
		return err
	default:
		if src, err = parseFwd(args[1], false); err != nil {
			return err
		}
		if dst, err = parseFwd(args[2], false); err != nil {
			return err
		}
		_, err = c.ForwardPort(ctx, &waterfall_grpc.PortForwardRequest{
			Session: &waterfall_grpc.ForwardSession{Src: src, Dst: dst}, Rebind: true})
		return err
	}
}

func passthroughFn(ctx context.Context, cfn ClientFn, args []string) error {
	conn, err := cfn()
	if err != nil {
		return err
	}
	defer conn.Close()

	_, err = exeStdout(ctx, waterfall_grpc.NewWaterfallClient(conn), args[0], args[1:]...)
	return err
}

func uninstallFn(ctx context.Context, cfn ClientFn, args []string) error {
	if len(args) != 2 {
		return ParseError{}
	}

	conn, err := cfn()
	if err != nil {
		return err
	}
	defer conn.Close()

	return shellStdout(ctx, waterfall_grpc.NewWaterfallClient(conn), "/system/bin/pm", args...)
}

func bugreportFn(ctx context.Context, cfn ClientFn, args []string) error {
	conn, err := cfn()
	if err != nil {
		return err
	}
	defer conn.Close()

	// bugreport [PATH]
	if len(args) > 2 {
		return ParseError{}
	}

	c := waterfall_grpc.NewWaterfallClient(conn)
	if len(args) == 1 {
		_, err = exeStdout(ctx, c, args[0], args[1:]...)
		return err
	}

	_, out, err := exeString(ctx, c, nil, "/system/bin/bugreportz", "-v")
	if err != nil || out == "" {
		fmt.Println("Bugreportz not supported. Use adb bugreport to run legacy version.")
		return nil
	}

	so := bytes.NewBuffer([]byte{})
	_, err = shellIO(ctx, c, so, nil, nil, "/system/bin/bugreportz")
	if err != nil {
		return err
	}

	oz := args[1]
	if !strings.HasSuffix(oz, ".zip") {
		oz += ".zip"
	}

	dp := strings.Split(so.String(), "OK:")
	if len(dp) < 2 {
		return fmt.Errorf("unable to parse bugreport zip output")
	}

	return client.Pull(ctx, c, strings.Trim(dp[1], "\n\r"), oz)
}

// ParseCommand parses the comand line args
func ParseCommand(args []string) (ParsedArgs, error) {
	var dev string

	// Process any global options first. Break and fallback for any unsupported option.
	i := 0
	done := false

	for !done && i < len(args)-1 {
		arg := args[i]
		switch arg {
		case "server":
			fallthrough
		case "nodaemon":
			fallthrough
		case "persist":
			fallthrough
		case "-p":
			fallthrough
		case "-a":
			fallthrough
		case "-e":
			fallthrough
		case "-d":
			fallthrough
		case "-t":
			return ParsedArgs{}, ParseError{}
		case "-s":
			dev = args[i+1]
			i++
		case "wait-for-device": // ignore
		// H, P and L have no meaning for H2O.
		case "-H":
			fallthrough
		case "-P":
			fallthrough
		case "-L":
			i++
		default:
			done = true
			continue
		}
		i++
	}
	return ParsedArgs{Device: dev, Command: args[i], Args: args[i:]}, nil
}
