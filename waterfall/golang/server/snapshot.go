// Copyright 2022 Google LLC
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

// Package snapshot lets waterfall record and check if it is running on a snapshotted emulator.
package snapshot

import (
	"context"
	"os/exec"
	"strings"
)

const snapshotProp = "persist.devx.snapshot"

func CreateSnapshotProp(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "setprop", snapshotProp)
	return cmd.Run()
}

func IsSnapshot(ctx context.Context) (bool, error) {
	cmd := exec.CommandContext(ctx, "getprop", snapshotProp)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false, err
	}
	return strings.HasPrefix(string(out), "1"), nil
}
