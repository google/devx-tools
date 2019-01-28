// adb_bin is a adb CLI compatible tool on top of H2O
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	empty_pb "github.com/golang/protobuf/ptypes/empty"
	"github.com/waterfall/client"
	waterfall_grpc "github.com/waterfall/proto/waterfall_go_grpc"
	"google.golang.org/grpc"
)

const (
	androidADBEnv = "ANDROID_ADB"
	androidSDKEnv = "ANDROID_SDK_HOME"
)

type parseError struct{}

// Error returns the empty string. We use this to fallback to regular ADB.
// This is only used to do a type assertion on the error.
func (e parseError) Error() string {
	return ""
}

type clientFn func() (*grpc.ClientConn, error)

type cmdFn func(context.Context, clientFn, []string) error

var (
	cmds = map[string]cmdFn{
		"shell":     shellFn,
		"push":      pushFn,
		"pull":      pullFn,
		"forward":   forwardFn,
		"bugreport": passthroughFn,
		"logcat":    passthroughFn,
		"install":   installFn,
		"uninstall": uninstallFn,
	}
)

func platformADB() (string, error) {
	p := os.Getenv("ANDROID_ADB")
	if p != "" {
		return p, nil
	}

	p = os.Getenv("ANDROID_SDK_HOME")
	if p != "" {
		return filepath.Join(p, "platform-tools/adb"), nil
	}
	return "", errors.New("unable to find platform ADB, neither ANDROID_ADB or ANDROID_SDK_HOME set")
}

// fallbackADB forks a new adb process and executes the provided command.
// Note that this method is terminal. We transfer control to adb.
// It will either fatal out or call Exit.
func fallbackADB(args []string) {
	adbBin, err := platformADB()
	if err != nil {
		log.Fatal(err)
	}

	cmd := exec.Command(adbBin, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGTERM}

	if err := cmd.Start(); err != nil {
		log.Fatal(err)
	}

	if err := cmd.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				os.Exit(status.ExitStatus())
			}
		}
		log.Fatal(err)
	}
	os.Exit(0)
}

func exe(ctx context.Context, c waterfall_grpc.WaterfallClient, cmd string, args ...string) (int, error) {
	// Pipe from stdin only if we pipes are being used
	return client.Exec(ctx, c, os.Stdout, os.Stderr, nil, cmd, args...)
}

func shell(ctx context.Context, c waterfall_grpc.WaterfallClient, cmd string, args ...string) error {
	// Ignore return code here. This is intended as a drop in replacement for adb, so we need to copy the behavior.
	_, err := exe(ctx, c, "/system/bin/sh", "-c", fmt.Sprintf("%s %s", cmd, strings.Join(args, " ")))
	return err
}

func shellFn(ctx context.Context, cfn clientFn, args []string) error {
	if len(args) == 1 {
		// Not an actual error, but user is requesting an interactive shell session.
		// Return this in order to fallback to adb.
		return parseError{}
	}
	conn, err := cfn()
	if err != nil {
		return err
	}
	defer conn.Close()
	return shell(ctx, waterfall_grpc.NewWaterfallClient(conn), args[1], args[2:]...)
}

func pushFn(ctx context.Context, cfn clientFn, args []string) error {
	if len(args) != 3 {
		return parseError{}
	}

	conn, err := cfn()
	if err != nil {
		return err
	}
	defer conn.Close()

	return client.Push(ctx, waterfall_grpc.NewWaterfallClient(conn), args[1], args[2])
}

func pullFn(ctx context.Context, cfn clientFn, args []string) error {
	if len(args) != 3 {
		return parseError{}
	}

	conn, err := cfn()
	if err != nil {
		return err
	}
	defer conn.Close()

	return client.Pull(ctx, waterfall_grpc.NewWaterfallClient(conn), args[1], args[2])
}

func installFn(ctx context.Context, cfn clientFn, args []string) error {
	if len(args) < 2 {
		return parseError{}
	}

	conn, err := cfn()
	if err != nil {
		return err
	}
	defer conn.Close()

	c := waterfall_grpc.NewWaterfallClient(conn)
	path := args[len(args)-1]
	tmp := filepath.Join("/data/local/tmp")
	if err := client.Push(ctx, c, path, tmp); err != nil {
		return err
	}
	defer exe(ctx, c, "rm", "-f", filepath.Join(tmp, filepath.Base(path)))

	return shell(ctx, c, "/system/bin/pm", append(args[:len(args)-1], filepath.Join(tmp, filepath.Base(path)))...)
}

func parseFwd(addr string) (string, error) {
	pts := strings.Split(addr, ":")
	if len(pts) != 2 {
		return "", parseError{}
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
		return "", parseError{}
	}
}

func forwardFn(ctx context.Context, cfn clientFn, args []string) error {
	if len(args) < 2 || len(args) > 4 {
		return parseError{}
	}

	conn, err := cfn()
	if err != nil {
		return err
	}
	defer conn.Close()
	c := waterfall_grpc.NewPortForwarderClient(conn)

	var src, dst string
	switch args[1] {
	case "--remove":
		if len(args) != 3 {
			return parseError{}
		}
		fwd, err := parseFwd(args[2])
		if err != nil {
			return err
		}
		_, err = c.Stop(ctx, &waterfall_grpc.PortForwardRequest{Src: fwd})
		return err
	case "--remove-all":
		if len(args) != 2 {
			return parseError{}
		}
		_, err = c.StopAll(ctx, &empty_pb.Empty{})
		return err
	case "--no-rebind":
		if src, err = parseFwd(args[2]); err != nil {
			return err
		}
		if dst, err = parseFwd(args[3]); err != nil {
			return err
		}
		_, err = c.ForwardPort(ctx, &waterfall_grpc.PortForwardRequest{Src: src, Dst: dst, Rebind: false})
		return err
	default:
		if src, err = parseFwd(args[1]); err != nil {
			return err
		}
		if dst, err = parseFwd(args[2]); err != nil {
			return err
		}
		_, err = c.ForwardPort(ctx, &waterfall_grpc.PortForwardRequest{Src: src, Dst: dst, Rebind: true})
		return err
	}
}

func passthroughFn(ctx context.Context, cfn clientFn, args []string) error {
	conn, err := cfn()
	if err != nil {
		return err
	}
	defer conn.Close()

	_, err = exe(ctx, waterfall_grpc.NewWaterfallClient(conn), args[0], args[1:]...)
	return err
}

func uninstallFn(ctx context.Context, cfn clientFn, args []string) error {
	if len(args) != 2 {
		return parseError{}
	}

	conn, err := cfn()
	if err != nil {
		return err
	}
	defer conn.Close()

	return shell(ctx, waterfall_grpc.NewWaterfallClient(conn), "/system/bin/pm", args...)
}

func runCommand(ctx context.Context, args []string) error {
	originalArgs := args[:]
	var deviceName string

	// Process any global options first. Break and fallback for any unsupported option.
	i := 0
	done := false

	for !done {
		arg := args[i]
		switch arg {
		case "server":
			fallthrough
		case "nodaemon":
			fallthrough
		case "persist":
			fallthrough
		case "wait-for-device":
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
			fallbackADB(args)
		case "-s":
			if len(args) == i+1 {
				return fmt.Errorf("error parsing command %v", args)
			}
			deviceName = args[i+1]
			i++
		// H, P and L have no meaning for H2O.
		case "-H":
			i++
		case "-P":
			i++
		case "-L":
			i++
		default:
			done = true
			continue
		}
		i++
	}
	args = args[i:]
	if len(args) > 0 {
		if deviceName == "" {
			// Let adb try to figure out if there is only one device.
			fallbackADB(originalArgs)
		}

		// We do dial outside gRPC domain in order to fallback to regular adb without grpc intervention
		cc, err := net.Dial("unix", "@h2o_"+deviceName)
		if err != nil {
			fallbackADB(originalArgs)
		}

		if fn, ok := cmds[args[0]]; ok {

			cfn := func() (*grpc.ClientConn, error) {
				return grpc.Dial(
					"",
					grpc.WithDialer(func(addr string, t time.Duration) (net.Conn, error) {
						return cc, nil
					}),
					grpc.WithBlock(),
					grpc.WithInsecure())
			}

			// Forward request uses a different service, so we need to dial accordingly.
			if args[0] == "forward" {
				cfn = func() (*grpc.ClientConn, error) {
					return grpc.Dial(fmt.Sprintf("unix:@h2o_%s_xforward", deviceName), grpc.WithInsecure())
				}
			}

			err := fn(ctx, cfn, args)
			if err != nil {
				if _, ok := err.(parseError); ok {
					fallbackADB(originalArgs)
				}
			}
			return err
		}
		// Fallback for any command we don't recognize
		fallbackADB(originalArgs)
	}
	fallbackADB(originalArgs)

	// unreachable
	return nil
}

func main() {
	if err := runCommand(context.Background(), os.Args[1:]); err != nil {
		log.Fatalf("Failed to execute adb command: %v", err)
	}
}
