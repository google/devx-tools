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
bazel test //waterfall:tar_test --test_output=streamed # --test_output=errors if this gets out of hand
bazel test //waterfall:stream_test --test_output=streamed # --test_output=errors if this gets out of hand
bazel test //waterfall/forward:stream_test --test_output=streamed # --test_output=errors if this gets out of hand
bazel test //waterfall/forward:forward_test --test_output=streamed # --test_output=errors if this gets out of hand

# TODO(mauriciogg): enable CI for this test. For now this needs to be ran
# manually since we need to dowload the ANDROID sdk to make it work
#bazel test //waterfall/net/qemu/... --test_output=streamed # --test_output=errors if this gets out of hand

