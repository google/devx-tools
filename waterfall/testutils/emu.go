package testutils

import (
	"context"
	"fmt"
	"io/ioutil"
	"os/exec"
	"strconv"
	"strings"
)

// startEmulator runs the emulator binary in the specified directory
func startEmulator(emuDir, launcher, adbServerPort, adbPort, emuPort string) error {
	// use mini_boot since we dont actually care about the android services
	// additionally pass --noenable_display and --nowith_audio so we can run inside
	// doker
	cmd := exec.Command(
		launcher, "--action", "mini_boot", "--emulator_tmp_dir",
		emuDir, "--adb_server_port", adbServerPort, "--adb_port", adbPort,
		"--emulator_port", emuPort, "--noenable_display", "--nowith_audio")

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error %v starting emulator: %s", err, out)
	}
	return nil
}

// KillEmu kills the emulator identified by adbPort.
func KillEmu(launcher, adbServerPort, adbPort, emuPort string) error {
	cmd := exec.Command(
		launcher, "--action", "kill", "--adb_server_port",
		adbServerPort, "--adb_port", adbPort, "--emulator_port", emuPort)
	_, err := cmd.CombinedOutput()
	return err
}

// ExecOnDevice runs the desired command on the device.
func ExecOnDevice(ctx context.Context, adbTurbo, device, cmd string, args []string) (string, error) {
	fullArgs := append([]string{"-s", device, cmd}, args...)
	if cmd == "shell" {
		fullArgs = []string{"-s", device, cmd, strings.Join(args, " ") + "; echo ret=$?"}
	}

	proc := exec.CommandContext(ctx, adbTurbo, fullArgs...)
	out, err := proc.CombinedOutput()

	o := string(out)
	if cmd == "shell" && err == nil {
		r := o[strings.LastIndex(o, "ret="):]
		o = strings.TrimSpace(o[:len(o)-len(r)])
		ret, convErr := strconv.Atoi(strings.TrimSpace(r[4:]))
		if convErr != nil {
			err = fmt.Errorf("error parsing return code %v", r)
		} else if ret != 0 {
			err = fmt.Errorf("non-zero return code '%d' out %s", ret, o)
		}
	}
	return o, err
}

// GetAdbPorts picks unused ports for the adb_port, adb_server_port and emulator_port.
func GetAdbPorts() (string, string, string, error) {
	p, err := pickUnusedPort()
	if err != nil {
		return "", "", "", err
	}
	adbServerPort := strconv.Itoa(p)

	p, err = pickUnusedPort()
	if err != nil {
		return "", "", "", err
	}
	adbPort := strconv.Itoa(p)

	p, err = pickUnusedPort()
	if err != nil {
		return "", "", "", err
	}
	emuPort := strconv.Itoa(p)

	return adbServerPort, adbPort, emuPort, nil
}

// SetupEmu starts up the emulator and returns the path to where is running.
func SetupEmu(launcher, adbServerPort, adbPort, emuPort string) (string, error) {
	emuDir, err := ioutil.TempDir("", "emulator")
	if err != nil {
		return "", err
	}
	if err := startEmulator(
		emuDir, launcher, adbServerPort, adbPort, emuPort); err != nil {
		return "", err
	}
	return emuDir, nil
}
