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
    deps = ["@com_github_google_gousb//:gousb"],
    importpath = "github.com/google/waterfall/golang/aoa",
    cgo = True,
)

# simple utility binary to manually test usb connection.
go_binary(
    name = "aoa_utils_bin",
    srcs = ["aoa_utils_bin.go"],
    deps = [
        ":aoa",
    ],
    cgo = True,
)

go_test(
    name = "aoa_test",
    srcs = [
        "aoa_test.go",  # TODO(mauriciogg): make this a a library dep
    ],
    deps = [
        "@org_golang_x_sync//errgroup:go_default_library",
    ],
    embed = [
        ":aoa",
    ],
    data = [
        "//waterfall/java/com/google/waterfall/usb:usb_service",
    ],
    importpath = "github.com/google/waterfall/golang/aoa",
)

