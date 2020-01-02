licenses(["notice"])  # Apache 2.0

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_binary",
    "go_test",
)

package(default_visibility = ["//visibility:public"])

go_binary(
    name = "forkfd_arm64",
    srcs = ["forkfd.go"],
    goarch = "arm64",
    goos = "linux",
    pure = "on",
    static = "on",
)
