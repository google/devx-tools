licenses(["notice"])  # Apache 2.0

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_binary",
    "go_library",
    "go_test",
)

package(default_visibility = ["//visibility:public"])

DEPS = [
        ":forward",
        "//waterfall/net/qemu",
]

# Host binary
go_binary(
    name = "forward_bin_amd64",
    srcs = ["forward_bin.go"],
    deps = DEPS,
)

# Emulator binaries
go_binary(
    name = "forward_bin_386",
    srcs = ["forward_bin.go"],
    goarch = "386",
    goos = "linux",
    pure = "on",
    static = "on",
    deps = DEPS,
)

go_binary(
    name = "forward_bin_arm",
    srcs = ["forward_bin.go"],
    goarch = "arm",
    goos = "linux",
    pure = "on",
    static = "on",
    deps = DEPS,
)

go_library(
    name = "forward",
    srcs = [
        "forward.go",
        "stream.go",
    ],
    deps = [
        "//waterfall",
        "//waterfall/adb",
        "//waterfall/client",
        "//waterfall/net/qemu",
        "//waterfall/proto:waterfall_go_grpc",
        "//waterfall/proto:waterfall_go_proto",
        "@org_golang_google_grpc//:go_default_library",
    ],
    importpath = "github.com/waterfall/forward",
)

go_test(
    name = "stream_test",
    srcs = [
        "forward.go",
        "stream.go",
        "stream_test.go",
    ],
    deps = [
        "//waterfall",
        "//waterfall/proto:waterfall_go_grpc",
        "//waterfall/proto:waterfall_go_proto",
    ],
    importpath = "github.com/waterfall/forward",
)

# integration test for the forward binary
go_test(
    name = "forward_test",
    srcs = [
        "forward_test.go",
    ],
    args = [
        "--fwdr_bin=$(location :forward_bin_amd64)",
    ],
    data = [
        "//waterfall/forward:forward_bin_amd64",
    ],
    deps = [
        "//waterfall/testutils:testutils",
    ],
    importpath = "github.com/waterfall/forward",
)
