# Registry: a simple in memory key value store accessible throu grpc

licenses(["notice"])  # Apache 2.0

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_binary",
    "go_library",
)

package(default_visibility = ["//visibility:public"])

go_library(
    name = "registry",
    srcs = ["registry.go"],
    importpath = "github.com/waterfall/registry",
    deps = [
        "//waterfall/proto:waterfall_go_grpc",
        "//waterfall/proto:waterfall_go_proto",
    ],
)
go_binary(
    name = "registry_bin",
    srcs = ["registry_bin.go"],
    deps = [
        ":registry",
        "//waterfall/proto:waterfall_go_grpc",
        "//waterfall/proto:waterfall_go_proto",
        "@org_golang_google_grpc//:go_default_library",
    ],
)
