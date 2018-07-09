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
        "--server=$(location //waterfall/server:server_bin)",
        "--launcher=$(location @android_test_support//tools/android/emulated_devices/generic_phone:android_24_x86)",
        "--adb_turbo=$(location @android_test_support//tools/android/emulator:support/adb.turbo)",
    ],
    data = [
        "//waterfall/server:server_bin",
        "@android_test_support//tools/android/emulated_devices/generic_phone:android_24_x86",
        "@android_test_support//tools/android/emulator:support/adb.turbo",
    ],
    importpath = "github.com/waterfall/server/testing",
    deps = [
        "//waterfall/client",
        "//waterfall/stream",
        "//waterfall/net/qemu",
        "//waterfall/testutils",
        "//waterfall/proto:waterfall_go_grpc",
        "@org_golang_google_grpc//:go_default_library",
        "@org_golang_x_sync//errgroup:go_default_library",
    ],
)
