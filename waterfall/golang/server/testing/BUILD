# Description:
#  Integration tests for the gRPC server

licenses(["notice"])  # Apache 2.0

load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_test(
    name = "qemu_server_test",
    srcs = [
        "qemu_server_test.go",
    ],
    args = [
        "--server=$(location //waterfall/golang/server:server_bin_386)",
        "--launcher=$(location @android_test_support//tools/android/emulated_devices/generic_phone:android_24_x86)",
        "--adb_turbo=$(location @android_test_support//tools/android/emulator:support/adb.turbo)",
    ],
    data = [
        "//waterfall/golang/server:server_bin_386",
        "@android_test_support//tools/android/emulated_devices/generic_phone:android_24_x86",
        "@android_test_support//tools/android/emulator:support/adb.turbo",
    ],
    importpath = "github.com/google/waterfall/golang/server/testing",
    deps = [
        "//waterfall/golang/client",
        "//waterfall/golang/net/qemu",
        "//waterfall/golang/testutils",
        "//waterfall/proto:waterfall_go_grpc",
        "@org_golang_google_grpc//:go_default_library",
        "@org_golang_x_sync//errgroup:go_default_library",
    ],
)
