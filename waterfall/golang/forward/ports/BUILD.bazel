licenses(["notice"])  # Apache 2.0

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_binary",
    "go_library",
    "go_test",
)

package(default_visibility = ["//visibility:public"])

go_binary(
    name = "ports_bin",
    srcs = [
        "ports_bin.go",
    ],
    pure = "on",
    static = "on",
    deps = [
        "//waterfall/golang/forward/ports",
        "//waterfall/proto:waterfall_go_grpc",
        "@org_golang_google_grpc//:go_default_library",
    ],
)

go_library(
    name = "ports",
    srcs = [
        "ports.go",
    ],
    importpath = "github.com/google/waterfall/golang/forward/ports",
    deps = [
        "@io_bazel_rules_go//proto/wkt:empty_go_proto",
        "@org_golang_google_grpc//codes:go_default_library",
        "@org_golang_google_grpc//status:go_default_library",
        "//waterfall/golang/forward",
        "//waterfall/proto:waterfall_go_grpc",
        "//waterfall/proto:waterfall_go_proto",
    ],
)

go_test(
    name = "ports_test",
    srcs = [
        "ports.go",
        "ports_test.go",
    ],
    importpath = "github.com/google/waterfall/golang/forward/ports",
    deps = [
        "@io_bazel_rules_go//proto/wkt:empty_go_proto",
        "@org_golang_google_grpc//:go_default_library",
        "@org_golang_google_grpc//codes:go_default_library",
        "@org_golang_google_grpc//status:go_default_library",
        "//waterfall/proto:waterfall_go_grpc",
        "//waterfall/proto:waterfall_go_proto",
        "//waterfall/golang/forward",
        "//waterfall/golang/server",
        "//waterfall/golang/testutils",
    ],
)
