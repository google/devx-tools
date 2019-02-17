workspace(name = "devx")

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

# Maven rule for transitive dependencies
RULES_MAVEN_TAG = "0.1.0" # or latest tag

http_archive(
    name = "rules_maven",
    strip_prefix = "rules_maven-%s" % RULES_MAVEN_TAG,
    url = "https://github.com/jin/rules_maven/archive/%s.zip" % RULES_MAVEN_TAG,
)

load("@rules_maven//:defs.bzl", "maven_install")

# Go toolchains
http_archive(
    name = "io_bazel_rules_go",
    url = "https://github.com/bazelbuild/rules_go/releases/download/0.16.5/rules_go-0.16.5.tar.gz",
    sha256 = "7be7dc01f1e0afdba6c8eb2b43d2fa01c743be1b9273ab1eaf6c233df078d705",
)

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_rules_dependencies",
    "go_register_toolchains",
)

go_rules_dependencies()
go_register_toolchains()

http_archive(
    name = "bazel_gazelle",
    urls = ["https://github.com/bazelbuild/bazel-gazelle/releases/download/0.15.0/bazel-gazelle-0.15.0.tar.gz"],
    sha256 = "6e875ab4b6bf64a38c352887760f21203ab054676d9c1b274963907e0768740d",
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
    sha256 = "48425cd631afb117fd355fd961deb313b3ac8e43f2b95c1598f35fbfcf684fbc",
    strip_prefix = "grpc-java-1.16.1",
    urls = ["https://github.com/grpc/grpc-java/archive/v1.16.1.tar.gz"],
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
