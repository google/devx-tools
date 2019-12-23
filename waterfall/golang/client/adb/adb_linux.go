package adb

import (
	"syscall"
)

var (
	procAttrs = &syscall.SysProcAttr{Pdeathsig: syscall.SIGTERM}
)
