// Copyright 2019 Google LLC
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

// Package adb implements a thin adb client to bootstrap h2o.
package adb

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

const (
	socketDirProp = "qemu.host.socket.dir"
	abiListProp   = "ro.product.cpu.abilist"
	abstractPort  = "localabstract:"
)

// Device implements functionality to talk to an Android device through ADB.
type Device struct {
	AdbPath       string
	DeviceName    string
	AdbServerPort string
	args          []string
	argsOnce      sync.Once
}

func (a *Device) deviceArgs() []string {
	a.argsOnce.Do(func() {
		args := []string{"-s", a.DeviceName}
		if a.AdbServerPort != "" {
			args = append(args, "-P", a.AdbServerPort)
		}
		a.args = args
	})
	return a.args
}

// Shell executes a command on the device.
func (a *Device) Shell(args []string) (string, error) {
	fullArgs := append(a.deviceArgs(), "shell", strings.Join(args, " ")+"; echo ret=$?")
	cmd := exec.Command(a.AdbPath, fullArgs...)
	out, err := cmd.CombinedOutput()
	o := string(out)

	if err != nil {
		return o, err
	}

	// Parse the exit status
	r := o[strings.LastIndex(o, "ret="):]
	o = strings.TrimSpace(o[:len(o)-len(r)])
	if ret, err := strconv.Atoi(strings.TrimSpace(r[4:])); err != nil {
		return "", fmt.Errorf("error parsing return code %v", r)
	} else if ret != 0 {
		return "", fmt.Errorf("non-zero return code %v = %s", args, o)
	}
	return o, nil
}

// Connect tries to connect the device to ADB.
func (a *Device) Connect() error {
	// Ignore error, verify the device is connected.
	exec.Command(a.AdbPath, append(a.deviceArgs(), "connect", a.DeviceName)...).CombinedOutput()

	// adb connect is actually verified by the follwowing code
	ob, err := exec.Command(a.AdbPath, append(a.deviceArgs(), "devices")...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("got error running adb devices: %s %v", string(ob), err)
	}

	// adb devices sample output:
	// List of devices attached
	// 127.0.0.1:40001	device

	out := string(ob)
	ls := strings.Split(out, "\n")
	for _, l := range ls {
		if strings.Contains(l, a.DeviceName) && strings.Contains(l, "device") {
			return nil
		}
	}
	return fmt.Errorf("%s not connected. devices output:\n %s", a.DeviceName, out)
}

// Push pushes a file from the host to the device.
func (a *Device) Push(src, dst string) error {
	cmd := exec.Command(a.AdbPath, append(a.deviceArgs(), "push", src, dst)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("error pushing %s - %s", src, string(out))
	}
	return nil
}

// QemuPipeDir returns the directory where the qemu sockets live if qemu pipes are supported.
func (a *Device) QemuPipeDir() (string, error) {
	p, err := a.Shell([]string{"getprop", socketDirProp})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(p), nil
}

// AbiList returns the list of supported ABI's
func (a *Device) AbiList() (string, error) {
	p, err := a.Shell([]string{"getprop", abiListProp})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(p), nil
}

// ForwardAbstract forwards a unix abstract socket from the host to the device.
func (a *Device) ForwardAbstract(local, remote string) error {
	cmd := exec.Command(a.AdbPath, append(a.deviceArgs(), "forward", abstractPort+local, abstractPort+remote)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("error forwarding %s -> %s %s", local, remote, string(out))
	}
	return nil
}

// StartCmd starts a new command on the device without waitin for it to finish.
func (a *Device) StartCmd(script string) error {
	cmd := exec.Command(a.AdbPath, append(a.deviceArgs(), "shell", script)...)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("error running script %s %v", script, err)
	}
	cmd.Process.Release()
	return nil
}
