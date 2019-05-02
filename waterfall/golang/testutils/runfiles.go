// Copyright 2018 Google LLC
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

package testutils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const workspaceDir = "runfiles/devx"

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
			if strings.HasSuffix(wd, workspaceDir) {
				runfiles = wd
				return
			}
		}

		execRoot := fmt.Sprintf("%s.%s", filepath.Base(os.Args[0]), workspaceDir)

		runfiles = wd[:strings.LastIndex(wd, execRoot)+len(execRoot)]

	})
	return runfiles
}
