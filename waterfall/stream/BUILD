licenses(["notice"])  # Apache 2.0

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_library",
    "go_test",
)

package(default_visibility = ["//visibility:public"])

go_library(
    name = "stream",
    srcs = ["tar.go"],
    importpath = "github.com/waterfall/stream",
)

go_test(
    name = "stream_test",
    srcs = [
        "tar.go",  # TODO(mauriciogg): make this a a library dep
        "tar_test.go",
    ],
    importpath = "github.com/waterfall/stream",
)
