licenses(["notice"])  # Apache 2.0

load(
    "@io_grpc_grpc_java//:java_grpc_library.bzl",
    "java_grpc_library",
)
load(
    "@io_bazel_rules_go//proto:def.bzl",
    "go_proto_library",
    "go_grpc_library",
)

package(default_visibility = ["//visibility:public"])

proto_library(
    name = "waterfall_proto",
    srcs = ["waterfall.proto"],
    deps = ["@com_google_protobuf//:empty_proto"],
)

go_proto_library(
    name = "waterfall_go_proto",
    importpath = "github.com/google/waterfall/proto/waterfall_go_proto",
    proto = ":waterfall_proto",
)

go_grpc_library(
    name = "waterfall_go_grpc",
    importpath = "github.com/google/waterfall/proto/waterfall_go_grpc",
    proto = ":waterfall_proto",
    deps = [":waterfall_go_proto"],
)

java_proto_library(
    name = "waterfall_java_proto",
    deps = [":waterfall_proto"],
)

java_grpc_library(
    name = "waterfall_java_grpc",
    srcs = [":waterfall_proto"],
    deps = [":waterfall_java_proto"],
)
