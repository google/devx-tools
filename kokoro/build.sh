#!/bin/bash -eu
#
# Copyright 2018 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

#!/bin/bash

# Get and install bazel
set -e

readonly BAZEL_VERSION="0.14.0"
readonly BAZEL_TMP=$(mktemp -d)
readonly BAZEL_INSTALLER_URL="https://github.com/bazelbuild/bazel/releases/download/${BAZEL_VERSION}/bazel-${BAZEL_VERSION}-installer-linux-x86_64.sh"
readonly BAZEL_INSTALLER="${BAZEL_TMP}/bazel-installer-linux.sh"

# Root directory of waterfall src
readonly CUR_DIR=$(dirname $(readlink -f "${0}"))
readonly ROOT_DIR=$(readlink -f "${CUR_DIR}/..")

curl -L  "${BAZEL_INSTALLER_URL}" > "${BAZEL_INSTALLER}"

# Install Bazel to /home/kbuilder/bin
bash "${BAZEL_INSTALLER}" --user &> /dev/null

export PATH="${PATH}:${HOME}/bin"

cd "${ROOT_DIR}"

# Test non emulator tests for now. Eventually will need to run on KVM instaces.
bazel test //waterfall/golang/stream:tar_test --test_output=streamed # --test_output=errors if this gets out of hand
bazel test //waterfall/golang/stream:stream_test --test_output=streamed # --test_output=errors if this gets out of hand
bazel test //waterfall/golang/forward:stream_test --test_output=streamed # --test_output=errors if this gets out of hand
bazel test //waterfall/golang/forward:forward_test --test_output=streamed # --test_output=errors if this gets out of hand
bazel test //waterfall/javatests/com/google/waterfall/tar --test_output=streamed # --test_output=errors if this gets out of hand
bazel test //waterfall/javatests/com/google/waterfall/client --test_output=streamed # --test_output=errors if this gets out of hand

# TODO(mauriciogg): enable CI for this test. For now this needs to be ran
# manually since we need to dowload the ANDROID sdk to make it work
#bazel test //waterfall/net/qemu/... --test_output=streamed # --test_output=errors if this gets out of hand

