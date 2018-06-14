licenses(["notice"])  # Apache 2.0

load(
    "@io_bazel_rules_go//proto:def.bzl",
    "go_proto_library",
    "go_grpc_library",
)

package(default_visibility = ["//visibility:public"])
proto_library(
    name = "waterfall_proto",
    srcs = ["waterfall.proto"],
)

go_proto_library(
    name = "waterfall_go_pb",
    importpath = "github.com/waterfall/proto/waterfall_go_pb",
    proto = ":waterfall_proto",
)

go_grpc_library(
    name = "waterfall_go_grpc",
    importpath = "github.com/waterfall/proto/waterfall_go_grpc",
    proto = ":waterfall_proto",
    deps = [":waterfall_go_pb"],
)

