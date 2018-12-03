licenses(["notice"])  # Apache 2.0

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_binary",
    "go_library",
    "go_test",
)

package(default_visibility = ["//visibility:public"])

go_binary(
    name = "bootstrap_bin",
    srcs = [
        "bootstrap_bin.go",
    ],
    deps = [
        ":bootstrap",
        "//waterfall/adb",
    ],
)

go_library(
    name = "bootstrap",
    srcs = [
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
    importpath = "github.com/waterfall/bootstrap",
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
        "--forwarder_bin=$(location //waterfall/forward:forward_bin_amd64)",
        "--adb_bin=$(location @androidsdk//:adb)",
        "--launcher_bin=$(location @android_test_support//tools/android/emulated_devices/generic_phone:android_24_x86)",
    ],
    data = [
        "//waterfall/server:server_bin_386",
        "//waterfall/forward:forward_bin_amd64",
        "@androidsdk//:adb",
        "@android_test_support//tools/android/emulated_devices/generic_phone:android_24_x86",
    ],
    deps = [
        "//waterfall/adb",
        "//waterfall/client",
        "//waterfall/forward",
        "//waterfall/net/qemu",
        "//waterfall/testutils:testutils",
        "//waterfall/proto:waterfall_go_grpc",
        "//waterfall/proto:waterfall_go_proto",
        "@org_golang_google_grpc//:go_default_library",
    ],
    importpath = "github.com/waterfall/bootstrap",
)
