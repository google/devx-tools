licenses(["notice"])  # Apache 2.0

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_binary",
    "go_library",
)

package(default_visibility = ["//visibility:public"])

go_binary(
    name = "forward_bin",
    srcs = [
        "forward_bin.go",
    ],
    deps = [
        "//waterfall/net/qemu",
    ],
)
