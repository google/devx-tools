package testutils

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const workspaceDir = "runfiles/__main__"

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

		if strings.HasSuffix(wd, workspaceDir) {
			// We are executing inside the runfiles dir
			runfiles = wd
			return
		}

		sep := filepath.Base(os.Args[0]) + "." + workspaceDir
		runfiles = wd[:strings.LastIndex(wd, sep)+len(sep)]

	})
	return runfiles
}
