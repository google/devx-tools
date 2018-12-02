licenses(["notice"])  # Apache 2.0

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_binary",
    "go_library",
    "go_test",
)

package(default_visibility = ["//visibility:public"])

go_binary(
    name = "forward_bin",
    srcs = [
        "forward_bin.go",
    ],
    deps = [
        ":forward",
        "//waterfall/adb",
        "//waterfall/forward/ports",
        "//waterfall/net/qemu",
        "//waterfall/proto:waterfall_go_grpc",
        "@org_golang_google_grpc//:go_default_library",
    ],
)

go_library(
    name = "forward",
    srcs = [
        "forward.go",
        "stream.go",
        "bootstrap.go",
    ],
    deps = [
        "//waterfall",
        "//waterfall/adb",
        "//waterfall/client",
        "//waterfall/net/qemu",
        "//waterfall/proto:waterfall_go_grpc",
        "//waterfall/proto:waterfall_go_proto",
        "@org_golang_google_grpc//:go_default_library",
    ],
    importpath = "github.com/waterfall/forward",
)

go_test(
    name = "stream_test",
    srcs = [
        "forward.go",
        "stream.go",
        "stream_test.go",
    ],
    deps = [
        "//waterfall",
        "//waterfall/proto:waterfall_go_grpc",
        "//waterfall/proto:waterfall_go_proto",
    ],
    importpath = "github.com/waterfall/forward",
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
        "//waterfall/forward:forward_bin",
    ],
    deps = [
        "//waterfall/testutils:testutils",
    ],
    importpath = "github.com/waterfall/forward",
)

# integration test for the bootsrap process.
# Note that we cant really use adb turbo at this point
# since the bootstrap process will use real adb features.
go_test(
    name = "bootstrap_test",
    srcs = [
        "bootstrap.go",
        "bootstrap_test.go",
    ],
    args = [
        "--waterfall_bin=$(location //waterfall/server:server_bin_386)",
        "--forwarder_bin=$(location //waterfall/forward:forward_bin)",
        "--adb_bin=$(location @androidsdk//:adb)",
        "--launcher_bin=$(location @android_test_support//tools/android/emulated_devices/generic_phone:android_24_x86)",
    ],
    data = [
        "//waterfall/server:server_bin_386",
        "//waterfall/forward:forward_bin",
        "@androidsdk//:adb",
        "@android_test_support//tools/android/emulated_devices/generic_phone:android_24_x86",
    ],
    deps = [
        "//waterfall/testutils:testutils",
        "//waterfall/adb",
        "//waterfall/client",
        "//waterfall/net/qemu",
        "//waterfall/proto:waterfall_go_grpc",
        "//waterfall/proto:waterfall_go_proto",
        "@org_golang_google_grpc//:go_default_library",
    ],
    importpath = "github.com/waterfall/forward",
)
