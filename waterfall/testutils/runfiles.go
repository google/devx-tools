package testutils

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var (
	runfiles string
	rfsOnce  sync.Once
)

// RunfilesRoot returns the path to the test runfiles dir
func RunfilesRoot() string {
	rfsOnce.Do(func() {
		wd, err := os.Getwd()
		if err != nil {
			panic("unable to get wd")
		}
		sep := filepath.Base(os.Args[0]) + ".runfiles/__main__"
		runfiles = wd[:strings.LastIndex(wd, sep)+len(sep)]
	})
	return runfiles
}
