// aoa_test test the aoa integration between the host and target device.
// Note that this test can only be run locally and the caller has to pass the device serial and the adb path.
package aoa

import (
	"bytes"
	"context"
	"flag"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sync/errgroup"
)

const (
	usbServiceApk = "waterfall/java/com/google/waterfall/usb/usb_service.apk"
	serviceName   = "com.google.waterfall.usb/.UsbService"
	serviceAction = "echo"
)

var (
	device  = flag.String("device", "", "The device serial number.")
	adbPath = flag.String("adb_path", "/usr/bin/adb", "Path to the adb binary.")
)

func runfiles() string {
	dir := filepath.Base(os.Args[0]) + ".runfiles"
	return filepath.Join(strings.Split(os.Args[0], dir)[0], dir, "devx")
}

// TestConnect tests libusb can attach to a device and transfer some bytes.
func TestConnect(t *testing.T) {

	if *device == "" {
		t.Fatalf("Need to provide -device arg")
	}

	if _, err := os.Stat(*adbPath); os.IsNotExist(err) {
		t.Fatal(err)
	}

	apk := filepath.Join(runfiles(), usbServiceApk)
	if _, err := os.Stat(apk); os.IsNotExist(err) {
		t.Fatal(err)
	}

	cmd := exec.Command(*adbPath, "-s", *device, "install", "-r", "-g", apk)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Install err %v with output %s", err, string(out))
	}

	cmd = exec.Command(
		*adbPath, "-s", *device, "shell", "am",
		"startservice", "-n", serviceName, "-a", serviceAction)
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("am err %v with output %s", err, string(out))
	}

	Reset(*device)
	u, err := Connect(*device)
	if err != nil {
		t.Fatal(err)
	}
	defer u.Close()

	sz := 1266716 /* ~1.2MB an arbitrary largeish transfer */
	bs := make([]byte, sz)
	for i, _ := range bs {
		bs[i] = byte(i % 256)
	}

	sb := bytes.NewBuffer(bs)
	eg, _ := errgroup.WithContext(context.Background())
	eg.Go(func() error {
		_, err := io.Copy(u, sb)
		return err
	})

	rbs := make([]byte, 0, sz)
	db := bytes.NewBuffer(rbs)
	eg.Go(func() error {
		_, err := io.CopyN(db, u, int64(sz))
		return err
	})

	if err := eg.Wait(); err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(bs, db.Bytes()) {
		t.Fatalf("Transfered bytes != received bytes")
	}

}
