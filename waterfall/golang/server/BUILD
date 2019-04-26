licenses(["notice"])  # Apache 2.0

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_binary",
    "go_library",
)

package(default_visibility = ["//visibility:public"])

go_library(
    name = "server",
    srcs = [
        "provider.go",
        "server.go",
    ],
    importpath = "github.com/google/waterfall/golang/server",
    deps = [
        "//waterfall/golang/constants",
        "//waterfall/golang/forward",
        "//waterfall/golang/net/qemu",
        "//waterfall/golang/stream",
        "//waterfall/proto:waterfall_go_grpc",
        "//waterfall/proto:waterfall_go_proto",
        "@io_bazel_rules_go//proto/wkt:empty_go_proto",
        "@org_golang_google_grpc//:go_default_library",
        "@org_golang_google_grpc//codes:go_default_library",
        "@org_golang_google_grpc//credentials:go_default_library",
        "@org_golang_google_grpc//encoding/gzip:go_default_library",
        "@org_golang_google_grpc//status:go_default_library",
        "@org_golang_x_sync//errgroup:go_default_library",
    ],
)

DEPS = [
    ":server",
]

go_binary(
    name = "server_bin_386",
    srcs = ["server_bin.go"],
    # Attributes for an emulator binary
    goarch = "386",
    goos = "linux",
    pure = "on",
    static = "on",
    deps = DEPS,
)

go_binary(
    name = "server_bin_amd64",
    srcs = ["server_bin.go"],
    # Attributes for an emulator binary
    goarch = "amd64",
    goos = "linux",
    pure = "on",
    static = "on",
    deps = DEPS,
)

go_binary(
    name = "server_bin_arm",
    srcs = ["server_bin.go"],
    # Attributes for a physical device
    goarch = "arm",
    goos = "linux",
    pure = "on",
    static = "on",
    deps = DEPS,
)

go_binary(
    name = "server_bin_arm64",
    srcs = ["server_bin.go"],
    # Attributes for a physical device
    goarch = "arm64",
    goos = "linux",
    pure = "on",
    static = "on",
    deps = DEPS,
)
