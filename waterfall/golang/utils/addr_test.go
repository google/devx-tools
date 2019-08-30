package utils

import (
	"testing"
)

func TestMuxFDAddr(t *testing.T) {
	addr := "mux:fd:3"
	pa, err := ParseAddr(addr)
	if err != nil {
		t.Error(err)
		return
	}

	if pa.Kind != Mux {
		t.Errorf("Expected %s kind but got %s", Mux, pa.Kind)
	}

	if pa.MuxAddr == nil {
		t.Errorf("expected non-nil MuxAddr: %v", pa)
	}

	if pa.MuxAddr.Kind != FD {
		t.Errorf("Expected %s kind but got %s", FD, pa.MuxAddr.Kind)
	}

	if pa.MuxAddr.FD != 3 {
		t.Errorf("Expected %d fd but got %d", 3, pa.MuxAddr.FD)
	}
}

func TestMuxUnixAddr(t *testing.T) {
	addr := "mux:unix:sockets/hello"
	pa, err := ParseAddr(addr)
	if err != nil {
		t.Error(err)
		return
	}

	if pa.Kind != Mux {
		t.Errorf("Expected %s kind but got %s", Mux, pa.Kind)
	}

	if pa.MuxAddr == nil {
		t.Errorf("expected non-nil MuxAddr: %v", pa)
	}

	if pa.MuxAddr.Kind != Unix {
		t.Errorf("Expected %s kind but got %s", Unix, pa.MuxAddr.Kind)
	}

	if pa.MuxAddr.Addr != "sockets/hello" {
		t.Errorf("Expected %s but got %s", "sockets/hello", pa.MuxAddr.Addr)
	}
}

func TestUnixAddr(t *testing.T) {
	addr := "unix:sockets/hello"
	pa, err := ParseAddr(addr)
	if err != nil {
		t.Error(err)
		return
	}

	if pa.Kind != Unix {
		t.Errorf("Expected %s kind but got %s", Unix, pa.Kind)
	}

	if pa.Addr != "sockets/hello" {
		t.Errorf("Expected %s but got %s", "sockets/hello", pa.Addr)
	}
}

func TestTCPAddr(t *testing.T) {
	addr := "tcp:localhost:9090"
	pa, err := ParseAddr(addr)
	if err != nil {
		t.Error(err)
		return
	}

	if pa.Kind != TCP {
		t.Errorf("Expected %s kind but got %s", TCP, pa.Kind)
	}

	if pa.Addr != "localhost:9090" {
		t.Errorf("Expected %s but got %s", "localhost:9090", pa.Addr)
	}
}

func TestQemuHostAddr(t *testing.T) {
	addr := "qemu:/foo/bar:sockets/hello"
	pa, err := ParseAddr(addr)
	if err != nil {
		t.Error(err)
		return
	}

	if pa.Kind != QemuHost {
		t.Errorf("Expected %s kind but got %s", QemuHost, pa.Kind)
	}

	if pa.Addr != "/foo/bar" {
		t.Errorf("Expected %s but got %s", "/foo/bar", pa.Addr)
	}

	if pa.SocketName != "sockets/hello" {
		t.Errorf("Expected %s but got %s", "sockets/hello", pa.SocketName)
	}
}

func TestQemuGuestAddr(t *testing.T) {
	addr := "qemu-guest:sockets/hello"
	pa, err := ParseAddr(addr)
	if err != nil {
		t.Error(err)
		return
	}

	if pa.Kind != QemuGuest {
		t.Errorf("Expected %s kind but got %s", QemuHost, pa.Kind)
	}

	if pa.SocketName != "sockets/hello" {
		t.Errorf("Expected %s but got %s", "sockets/hello", pa.SocketName)
	}
}

func TestInvalidKind(t *testing.T) {
	addr := "foo:sockets/hello"
	pa, err := ParseAddr(addr)
	if err == nil {
		t.Errorf("expected error but got %v", pa)
	}
}
