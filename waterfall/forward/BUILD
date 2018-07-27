licenses(["notice"])  # Apache 2.0

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_binary",
    "go_library",
    "go_test",
)

package(default_visibility = ["//visibility:public"])

go_binary(
    name = "forward_bin",
    srcs = [
        "forward_bin.go",
    ],
    deps = [
        ":forward",
        "//waterfall/net/qemu",
    ],
)

go_library(
    name = "forward",
    srcs = [
        "forward.go",
        "stream.go",
    ],
    deps = [
        "//waterfall",
        "//waterfall/proto:waterfall_go_grpc",
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
        "--fwdr=$(location :forward_bin)",
    ],
    data = [
        ":forward_bin",
    ],
    deps = [
        "//waterfall/testutils:testutils",
    ],
    importpath = "github.com/waterfall/forward",
)
