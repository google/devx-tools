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
        "//waterfall/golang/adb",
    ],
)

# Keep this binary isolated, since it has a dependency on libusb.
go_binary(
    name = "bootstrap_usb_bin",
    srcs = [
        "bootstrap_usb_bin.go",
    ],
    deps = [
        "//waterfall/golang/adb",
        "@org_golang_google_grpc//:go_default_library",
    ],
)

go_library(
    name = "bootstrap",
    srcs = [
        "bootstrap.go",
    ],
    deps = [
        "//waterfall/golang/adb",
        "//waterfall/golang/client",
        "//waterfall/golang/net/qemu",
        "//waterfall/proto:waterfall_go_grpc",
        "//waterfall/proto:waterfall_go_proto",
        "@org_golang_google_grpc//:go_default_library",
        "@org_golang_x_sync//errgroup:go_default_library",
    ],
    importpath = "github.com/google/waterfall/golang/bootstrap",
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
        "--waterfall_bin=$(location //waterfall/golang/server:server_bin_386)",
        "--forwarder_bin=$(location //waterfall/golang/forward:forward_bin)",
        "--adb_bin=$(location @androidsdk//:adb)",
        "--launcher_bin=$(location @android_test_support//tools/android/emulated_devices/generic_phone:android_23_x86)",
    ],
    data = [
        "//waterfall/golang/server:server_bin_386",
        "//waterfall/golang/forward:forward_bin",
        "@androidsdk//:adb",
        "@android_test_support//tools/android/emulated_devices/generic_phone:android_23_x86",
    ],
    deps = [
        "//waterfall/golang/adb",
        "//waterfall/golang/client",
        "//waterfall/golang/forward",
        "//waterfall/golang/net/qemu",
        "//waterfall/golang/testutils:testutils",
        "//waterfall/proto:waterfall_go_grpc",
        "//waterfall/proto:waterfall_go_proto",
        "@org_golang_google_grpc//:go_default_library",
        "@org_golang_x_sync//errgroup:go_default_library",
    ],
    importpath = "github.com/google/waterfall/golang/bootstrap",
)
