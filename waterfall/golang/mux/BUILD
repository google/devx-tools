licenses(["notice"])  # Apache 2.0

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_library",
    "go_test",
)

package(default_visibility = ["//visibility:public"])

go_library(
    name = "mux",
    srcs = [
        "addr.go",
        "conn.go",
        "mux.go",
        "message.go",
    ],
    importpath = "github.com/google/waterfall/golang/mux",
    deps = [
        "//waterfall/proto:waterfall_go_grpc",
        "//waterfall/golang/stream:stream",
        "@org_golang_google_grpc//:go_default_library",
    ],
)

go_test(
    name = "mux_test",
    srcs = [
        "mux_test.go",
    ],
    deps = [
        "@org_golang_x_sync//errgroup:go_default_library",
    ],
    embed = [
        ":mux",
    ],
    importpath = "github.com/google/waterfall/golang/mux",
)

