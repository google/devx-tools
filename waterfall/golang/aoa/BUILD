licenses(["notice"])  # Apache 2.0

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_library",
    "go_binary",
    "go_test",
)

package(default_visibility = ["//visibility:public"])

# In order to build this target libusb needs to be installed.
go_library(
    name = "aoa",
    srcs = ["aoa.go"],
    cgo = True,
    importpath = "github.com/google/waterfall/golang/aoa",
    deps = ["@com_github_google_gousb//:gousb"],
)

# simple utility binary to manually test usb connection.
go_binary(
    name = "aoa_utils_bin",
    srcs = ["aoa_utils_bin.go"],
    cgo = True,
    deps = [
        ":aoa",
    ],
)

go_test(
    name = "aoa_test",
    srcs = [
        "aoa_test.go",
    ],
    data = [
        "//waterfall/java/com/google/waterfall/usb:usb_service",
    ],
    embed = [
        ":aoa",
    ],
    importpath = "github.com/google/waterfall/golang/aoa",
    deps = [
        "@org_golang_x_sync//errgroup:go_default_library",
    ],
)
