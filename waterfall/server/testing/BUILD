#Description:
#  Integration tests for the gRPP server and custom connections
#  like qemu_pipes and usb

load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")


go_test(
    name = "qemu_server_test",
    srcs = [
        "qemu_test_client.go",
        "qemu_server_test.go",
    ],
    args = [
        "--server=$(location //waterfall/server:server)",
        "--launcher=$(location @android_test_support//tools/android/emulated_devices/generic_phone:android_24_x86)",
        "--adb_turbo=$(location @android_test_support//tools/android/emulator:support/adb.turbo)",
    ],
    data = [
        "//waterfall/server:server",
        "@android_test_support//tools/android/emulated_devices/generic_phone:android_24_x86",
        "@android_test_support//tools/android/emulator:support/adb.turbo",
    ],
    importpath = "github.com/waterfall/server/testing",
    deps = [
        "//waterfall/proto:waterfall_grpc",
        "//waterfall/stream",
        "//waterfall/net/qemu",
        "//waterfall/test_utils",
        "@org_golang_google_grpc//:go_default_library",
        "@org_golang_x_sync//errgroup:go_default_library",
    ],
)
