package testutils

import (
	"os"
	"path/filepath"
	"strings"
)

// RunfilesRoot returns the path to the test runfiles dir
func RunfilesRoot(path string) string {
	sep := filepath.Base(os.Args[0]) + ".runfiles/__main__"
	return path[:strings.LastIndex(path, sep)+len(sep)]
}
