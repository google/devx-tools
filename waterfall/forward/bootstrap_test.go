package forward

import (
	"bytes"
	"context"
	"flag"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/waterfall/adb"
	"github.com/waterfall/client"
	"github.com/waterfall/testutils"
	"google.golang.org/grpc"

	waterfall_grpc "github.com/waterfall/proto/waterfall_go_grpc"
)

var (
	adbBin       = flag.String("adb_bin", "", "The path to adb binary.")
	waterfallBin = flag.String("waterfall_bin", "", "The path to test server.")
	forwarderBin = flag.String("forwarder_bin", "", "The path to forwarder binary.")
	launcherBin  = flag.String("launcher_bin", "", "Path to the emulator launcher binary.")

	l      string
	a      string
	fwdr   string
	svrBin string
	svr    []string
)

const (
	socketName    = "sockets/h2o"
	socketDirProp = "qemu.host.socket.dir"
	prop          = "waterfall.test"
	propVal       = "foobarbazfefifofum"
)

func init() {
	flag.Parse()
	if *launcherBin == "" || *adbBin == "" || *waterfallBin == "" {
		log.Fatalf("launcher, adb_bin and waterfall_bin args need to be provided")
	}

	l = filepath.Join(testutils.RunfilesRoot(), *launcherBin)
	a = filepath.Join(testutils.RunfilesRoot(), *adbBin)
	fwdr = filepath.Join(testutils.RunfilesRoot(), *forwarderBin)
	svrBin = filepath.Join(testutils.RunfilesRoot(), *waterfallBin)
	svr = []string{"x86:" + svrBin}
}

func getProp(addr, prop string) (string, error) {
	cc, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		return "", err
	}

	wc := waterfall_grpc.NewWaterfallClient(cc)
	sOut := new(bytes.Buffer)
	if err := client.Exec(context.Background(), wc, sOut, ioutil.Discard, nil, "getprop", prop); err != nil {
		return "", err
	}
	return strings.TrimSpace(sOut.String()), nil
}

func initAdb(adbPath, adbPort, serverPort string) *adb.Device {
	adbConn := &adb.Device{
		AdbPath:       adbPath,
		DeviceName:    "127.0.0.1:" + adbPort,
		AdbServerPort: serverPort}

	// issue a connect and ignore the error. We are dealing with real adb at this point so
	// se need to make sure the adb daemon is started. Fun times!
	adbConn.Connect()
	return adbConn
}

func setSocketProp(a *adb.Device, emuDir string) error {
	_, err := a.Shell([]string{
		"setprop", socketDirProp, filepath.Join(emuDir, "images/session/sockets")})
	return err
}

func TestQemuNeedsBootstrap(t *testing.T) {
	adbServerPort, adbPort, emuPort, err := testutils.GetAdbPorts()
	if err != nil {
		t.Fatal(err)
	}

	emuDir, err := testutils.SetupEmu(l, adbServerPort, adbPort, emuPort)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(emuDir)
	defer testutils.KillEmu(l, adbServerPort, adbPort, emuPort)

	adbConn := initAdb(a, adbPort, adbServerPort)

	// Set the emulator dir property. Mini boot does not set it.
	if err := setSocketProp(adbConn, emuDir); err != nil {
		t.Fatalf("Unable to set qemu socket dir prop: %v", err)
	}

	r, err := Bootstrap(adbConn, svr, fwdr, socketName)
	if err != nil {
		t.Fatalf("Error during bootstrap %v", err)
	}

	if !r.StartedForwarder {
		t.Errorf("Forwader daemon not started!")
	}

	if out, err := adbConn.Shell([]string{"setprop", prop, propVal}); err != nil {
		t.Fatalf("Unable to set canary prop: %s %v", out, err)
	}

	out, err := getProp("unix:@h2o_"+adbConn.DeviceName, prop)
	if err != nil {
		t.Fatalf("Server ping failed %v", err)
	}
	if out != propVal {
		t.Fatalf("Got prop value %v but expected %v", out, propVal)
	}
}

func TestQemuNoBootsrapNeeded(t *testing.T) {
	adbServerPort, adbPort, emuPort, err := testutils.GetAdbPorts()
	if err != nil {
		t.Fatal(err)
	}

	emuDir, err := testutils.SetupEmu(l, adbServerPort, adbPort, emuPort)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(emuDir)
	defer testutils.KillEmu(l, adbServerPort, adbPort, emuPort)

	adbConn := initAdb(a, adbPort, adbServerPort)
	if err := startWaterfall(adbConn, svrBin, "qemu:"+socketName); err != nil {
		t.Fatalf("Unable to start waterfall server: %v", err)
	}

	if err := setSocketProp(adbConn, emuDir); err != nil {
		t.Fatalf("Unable to set qemu socket dir prop: %v", err)
	}

	r, err := Bootstrap(adbConn, svr, fwdr, socketName)
	if err != nil {
		t.Fatalf("Error during bootstrap %v", err)
	}

	// Second bootstrap should be a no-op
	r, err = Bootstrap(adbConn, svr, fwdr, socketName)
	if err != nil {
		t.Fatalf("Error during bootstrap %v", err)
	}

	if r.StartedServer {
		t.Errorf("Started server but it was already running!")
	}

	if r.StartedForwarder {
		t.Errorf("Forwader started but it was already running!")
	}

	if out, err := adbConn.Shell([]string{"setprop", prop, propVal}); err != nil {
		t.Fatalf("Unable to set canary prop: %s %v", out, err)
	}

	out, err := getProp("unix:@h2o_"+adbConn.DeviceName, prop)
	if err != nil {
		t.Fatalf("Server ping failed %v", err)
	}
	if out != propVal {
		t.Fatalf("Got prop value %v but expected %v", out, propVal)
	}
}

func TestAdbNeedsBootstrap(t *testing.T) {
	adbServerPort, adbPort, emuPort, err := testutils.GetAdbPorts()
	if err != nil {
		t.Fatal(err)
	}

	emuDir, err := testutils.SetupEmu(l, adbServerPort, adbPort, emuPort)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(emuDir)
	defer testutils.KillEmu(l, adbServerPort, adbPort, emuPort)

	adbConn := initAdb(a, adbPort, adbServerPort)

	// Don't set qemu prop so this gets treated as a physical device
	r, err := Bootstrap(adbConn, svr, fwdr, socketName)
	if err != nil {
		t.Fatalf("Error during bootstrap %v", err)
	}

	if r.StartedForwarder {
		t.Errorf("Forwader daemon started, but was supposed to tunnel through adb")
	}

	if out, err := adbConn.Shell([]string{"setprop", prop, propVal}); err != nil {
		t.Fatalf("Unable to set canary prop: %s %v", out, err)
	}

	out, err := getProp("unix:@h2o_"+adbConn.DeviceName, prop)
	if err != nil {
		t.Errorf("Server ping failed %v", err)
		return
	}
	if out != propVal {
		t.Errorf("Got prop value %v but expected %v", out, propVal)
		return
	}

}

func TestAdbServerIsRunning(t *testing.T) {
	adbServerPort, adbPort, emuPort, err := testutils.GetAdbPorts()
	if err != nil {
		t.Fatal(err)
	}

	emuDir, err := testutils.SetupEmu(l, adbServerPort, adbPort, emuPort)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(emuDir)
	defer testutils.KillEmu(l, adbServerPort, adbPort, emuPort)

	adbConn := initAdb(a, adbPort, adbServerPort)
	addr := "unix:@h2o_" + adbConn.DeviceName
	if err := startWaterfall(adbConn, svrBin, addr); err != nil {
		t.Fatalf("Unable to start waterfall server: %v", err)
	}
	// wait for the server to come up
	time.Sleep(time.Millisecond * 300)

	r, err := Bootstrap(adbConn, svr, fwdr, socketName)
	if err != nil {
		t.Fatalf("Error during bootstrap %v", err)
	}

	if r.StartedServer {
		t.Errorf("Started server but it was already running!")
	}

	if r.StartedForwarder {
		t.Errorf("Forwader daemon started, but was supposed to tunnel through adb")
	}

	if out, err := adbConn.Shell([]string{"setprop", prop, propVal}); err != nil {
		t.Fatalf("Unable to set canary prop: %s %v", out, err)
	}

	out, err := getProp(addr, prop)
	if err != nil {
		t.Fatalf("Server ping failed %v", err)
	}
	if out != propVal {
		t.Fatalf("Got prop value %v but expected %v", out, propVal)
	}
}
