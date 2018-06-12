licenses(["notice"])  # Apache 2.0

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_binary",
    "go_library",
)
package(default_visibility = ["//visibility:public"])
go_library(
    name = "server_lib",
    srcs = ["server.go"],
    importpath = "github.com/waterfall/server",
    deps = [
        "//waterfall/proto:waterfall_grpc",
        "//waterfall/proto:waterfall_proto_lib",
        "//waterfall/stream",
        "@org_golang_google_grpc//:go_default_library",
        "@org_golang_x_sync//errgroup:go_default_library",
    ],
)
go_binary(
    name = "server",
    srcs = ["server_bin.go"],
    # Attributes for an emulator binary
    goarch = "386",
    goos = "linux",
    pure = "on",
    static = "on",
    deps = [
        ":server_lib",
        "//waterfall/proto:waterfall_grpc",
        "//waterfall/proto:waterfall_proto_lib",
        "//waterfall/net/qemu",
        "@org_golang_google_grpc//:go_default_library",
    ],
)
