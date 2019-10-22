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
        "//waterfall/golang/mux",
        "//waterfall/golang/net/qemu",
        "//waterfall/golang/utils",
        "@org_golang_x_sync//errgroup:go_default_library",
]

# Host binary
go_binary(
    name = "forward_bin",
    srcs = ["forward_bin.go"],
    goos = "linux",
    pure = "on",
    static = "on",
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

# USB host binary
# Keep this isolated. It links against libusb and which has a GNU lesser licence.
go_binary(
    name = "forward_usb_bin",
    srcs = ["forward_usb_bin.go"],
    goos = "linux",
    cgo = "on",
    deps = DEPS + ["//waterfall/golang/aoa"],
)

go_library(
    name = "forward",
    srcs = [
        "forward.go",
        "stream.go",
    ],
    deps = [
        "//waterfall/golang/stream",
        "//waterfall/golang/adb",
        "//waterfall/golang/client",
        "//waterfall/golang/net/qemu",
        "//waterfall/proto:waterfall_go_grpc",
        "//waterfall/proto:waterfall_go_proto",
        "@org_golang_google_grpc//:go_default_library",
    ],
    importpath = "github.com/google/waterfall/golang/forward",
)

go_test(
    name = "stream_test",
    srcs = [
        "forward.go",
        "stream.go",
        "stream_test.go",
    ],
    deps = [
        "//waterfall/golang/stream",
        "//waterfall/proto:waterfall_go_grpc",
        "//waterfall/proto:waterfall_go_proto",
    ],
    importpath = "github.com/google/waterfall/golang/forward",
)

# integration test for the forward binary
go_test(
    name = "forward_test",
    srcs = [
        "forward_test.go",
    ],
    args = [
        "--fwdr_bin=$(location :forward_bin)",
    ],
    data = [
        "//waterfall/golang/forward:forward_bin",
    ],
    deps = [
        "//waterfall/golang/testutils:testutils",
    ],
    importpath = "github.com/google/waterfall/golang/forward",
)
