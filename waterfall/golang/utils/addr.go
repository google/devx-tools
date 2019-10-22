// Package utils implement utility functions used by several libs.
package utils

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	// QemuHost describes a an address to start a qemu socket from the host.
	// It is of the form qemu:$emu_dir:$socket_name where emu_dir is the
	// working dir of the emulator and socket_name is the name of the socket file that will be created.
	QemuHost = "qemu"

	// QemuGuest describes an address from the guest perspective.
	// It is of the form qemu-guest:$socket_name where socket_name is the name of the socket the host is listening.
	QemuGuest = "qemu-guest"

	// QemuCtrl describes an address from the guest perspective, when using the qemu2 protocol.
	// It is of the form qemu-guest:$socket_name where socket_name is the name of the socket the host is listening.
	QemuCtrl = "qemu2"

	// Unix describes a Unix socket address.
	// It is of the form unix:$socket_name. If socket_name contains a leading @ then this is an abstract socket.
	Unix = "unix"

	// TCP describes a TCP socket.
	// It is of the form tcp:$addr:$port
	TCP = "tcp"

	// Mux describes an address of a connection that will be multiplexed through the gRPC multiplexer service.
	// It is of the form mux:$addr where addr itself is an unparsed <kind>:<addr> address.
	// addr will be used to open a connection the Mux service and is only intended to serve a single connection.
	Mux = "mux"

	// FD describes a file descriptor.
	// It is of the form fd:$fd_num where fd_num is the file descriptor number to open.
	// Note that this is only intended to be used in conjuction with a Mux addr.
	FD = "fd"

	// USB describes a USB endpoint.
	// It is of the form usb:$usb_serial where usb_serial is the serial number of the usb device.
	// Note that this is only intended to be used in conjuction with a Mux addr.
	USB = "usb"
)

var (
	validKinds = map[string]bool{
		QemuHost:  true,
		QemuCtrl:  true,
		QemuGuest: true,
		Unix:      true,
		TCP:       true,
		Mux:       true,
		FD:        true,
		USB:       true,
	}
)

// ParsedAddr represents an address of the form <kind>:<addr>.
type ParsedAddr struct {
	Kind       string
	Addr       string
	SocketName string
	FD         int

	// The address to use for multiplexed connections.
	MuxAddr *ParsedAddr
}

// ParseAddr takes an address of the form <kind>:<addr> and returns a ParsedAddr.
func ParseAddr(addr string) (*ParsedAddr, error) {
	pts := strings.SplitN(addr, ":", 2)
	if len(pts) < 2 {
		return nil, fmt.Errorf("failed to parse address %s", addr)
	}

	if !validKinds[pts[0]] {
		return nil, fmt.Errorf("invalid kind: %s", pts[0])
	}

	if pts[0] == Mux {
		mAddr, err := ParseAddr(pts[1])
		if err != nil {
			return nil, err
		}
		return &ParsedAddr{Kind: pts[0], MuxAddr: mAddr}, nil
	}

	if pts[0] == FD {
		fd, err := strconv.Atoi(pts[1])
		if err != nil {
			return nil, err
		}
		return &ParsedAddr{Kind: pts[0], FD: fd}, nil
	}

	host := pts[1]
	socket := ""
	if pts[0] == QemuGuest || pts[0] == QemuCtrl {
		socket = pts[1]
	}

	if pts[0] == QemuHost {
		p := strings.SplitN(pts[1], ":", 2)
		if len(p) != 2 {
			return nil, fmt.Errorf("failed to parse address %s", addr)
		}
		host = p[0]
		socket = p[1]
	}
	return &ParsedAddr{Kind: pts[0], Addr: host, SocketName: socket}, nil
}
