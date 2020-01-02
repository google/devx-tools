# Description:
#   Package client is a reference waterfall client implementation

licenses(["notice"])  # Apache 2.0

load("@io_bazel_rules_go//go:def.bzl", "go_library")

package(default_visibility = ["//visibility:public"])

go_library(
    name = "client",
    srcs = [
        "client.go",
    ],
    importpath = "github.com/google/waterfall/golang/client",
    deps = [
        "//waterfall/golang/stream",
        "//waterfall/golang/net/qemu",
        "//waterfall/proto:waterfall_go_grpc",
        "//waterfall/proto:waterfall_go_proto",
        "@org_golang_google_grpc//:go_default_library",
        "@org_golang_x_sync//errgroup:go_default_library",
    ],
)
