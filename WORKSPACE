workspace(name = "devx")

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

# Maven rule for transitive dependencies
load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

RULES_JVM_EXTERNAL_TAG = "1.2"
RULES_JVM_EXTERNAL_SHA = "e5c68b87f750309a79f59c2b69ead5c3221ffa54ff9496306937bfa1c9c8c86b"

http_archive(
    name = "rules_jvm_external",
    strip_prefix = "rules_jvm_external-%s" % RULES_JVM_EXTERNAL_TAG,
    sha256 = RULES_JVM_EXTERNAL_SHA,
    url = "https://github.com/bazelbuild/rules_jvm_external/archive/%s.zip" % RULES_JVM_EXTERNAL_TAG,
)

load("@rules_jvm_external//:defs.bzl", "maven_install")

# Go toolchains
http_archive(
    name = "io_bazel_rules_go",
    urls = ["https://github.com/bazelbuild/rules_go/releases/download/0.18.3/rules_go-0.18.3.tar.gz"],
    sha256 = "86ae934bd4c43b99893fc64be9d9fc684b81461581df7ea8fc291c816f5ee8c5",
)

load(
    "@io_bazel_rules_go//go:deps.bzl",
    "go_rules_dependencies",
    "go_register_toolchains",
)

go_rules_dependencies()
go_register_toolchains()

http_archive(
    name = "bazel_gazelle",
    urls = ["https://github.com/bazelbuild/bazel-gazelle/releases/download/0.17.0/bazel-gazelle-0.17.0.tar.gz"],
    sha256 = "3c681998538231a2d24d0c07ed5a7658cb72bfb5fd4bf9911157c0e9ac6a2687",
)

load("@bazel_gazelle//:deps.bzl", "gazelle_dependencies", "go_repository")
gazelle_dependencies()

go_repository(
    name = "org_golang_x_sync",
    commit = "1d60e4601c6fd243af51cc01ddf169918a5407ca",
    importpath = "golang.org/x/sync",
)

# Java toolchains

# gRPC Java
http_archive(
    name = "io_grpc_grpc_java",
    sha256 = "0b86e44f9530fd61eb044b3c64c7579f21857ba96bcd9434046fd22891483a6d",
    strip_prefix = "grpc-java-1.18.0",
    urls = ["https://github.com/grpc/grpc-java/archive/v1.18.0.tar.gz"],
)

load("@io_grpc_grpc_java//:repositories.bzl", "grpc_java_repositories")

grpc_java_repositories(
    omit_com_google_protobuf = True,
)

maven_install(
    artifacts = [
        "org.junit.jupiter:junit-jupiter-engine:5.3.2",
        "io.grpc:grpc-testing:1.16.1",
        "io.grpc:grpc-all:1.16.1",
        "org.apache.commons:commons-compress:1.10",
    ],
    repositories = [
        "https://maven.google.com",
        "https://repo1.maven.org/maven2",
        "https://mvnrepository.com",
    ],
)

# Android libs
android_sdk_repository(name = "androidsdk")

ATS_COMMIT = "1daba70e7b5952fc3fb46b9bd99dd2f80c2bdaa3"
http_archive(
    name = "android_test_support",
    strip_prefix = "android-test-%s" % ATS_COMMIT,
    urls = ["https://github.com/android/android-test/archive/%s.tar.gz" % ATS_COMMIT],
)

load("@android_test_support//:repo.bzl", "android_test_repositories")
android_test_repositories()
