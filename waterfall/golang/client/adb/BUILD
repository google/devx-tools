# Description:
#  ADB command line compatible implementation of waterfall

licenses(["notice"])  # Apache 2.0

load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_binary")

package(default_visibility = ["//visibility:public"])

BINARY_DEPS = [
        ":adb",
        "@org_golang_google_grpc//:go_default_library",
]

go_binary(
    name = "adb_bin",
    srcs = [
        "adb_bin.go",
    ],
    static = "on",
    pure = "on",
    deps = BINARY_DEPS,
)

go_binary(
    name = "adb_bin_darwin",
    srcs = [
        "adb_bin.go",
    ],
    goos = "darwin",
    deps = BINARY_DEPS,
)

go_library(
    name = "adb",
    srcs = [
        "adb.go",
        "adb_linux.go",
        "adb_darwin.go",
    ],
    deps = [
        "@io_bazel_rules_go//proto/wkt:empty_go_proto",
        "//waterfall/golang/client",
        "//waterfall/proto:waterfall_go_grpc",
        "@org_golang_google_grpc//:go_default_library",
    ],
    importpath = "github.com/google/waterfall/golang/client/adb",
)
