workspace(name = "devx")

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

# Maven rule for transitive dependencies
RULES_JVM_EXTERNAL_TAG = "1.2"
RULES_JVM_EXTERNAL_SHA = "e5c68b87f750309a79f59c2b69ead5c3221ffa54ff9496306937bfa1c9c8c86b"

http_archive(
    name = "rules_jvm_external",
    strip_prefix = "rules_jvm_external-%s" % RULES_JVM_EXTERNAL_TAG,
    sha256 = RULES_JVM_EXTERNAL_SHA,
    url = "https://github.com/bazelbuild/rules_jvm_external/archive/%s.zip" % RULES_JVM_EXTERNAL_TAG,
)

load("@rules_jvm_external//:defs.bzl", "maven_install")

# gRPC Java
# Note that this needs to come before io_bazel_go_rules. Both depend on
# protobuf and the version that io_bazel_rules_go depends on is broken for
# java, so io_grpc_grpc_java needs to get the dep first.
http_archive(
    name = "io_grpc_grpc_java",
    sha256 = "9bc289e861c6118623fcb931044d843183c31d0e4d53fc43c4a32b56d6bb87fa",
    strip_prefix = "grpc-java-1.21.0",
    urls = ["https://github.com/grpc/grpc-java/archive/v1.21.0.tar.gz"],
)


load("@io_grpc_grpc_java//:repositories.bzl", "grpc_java_repositories")

grpc_java_repositories()

# Go toolchains
http_archive(
    name = "io_bazel_rules_go",
    url = "https://github.com/bazelbuild/rules_go/releases/download/0.18.4/rules_go-0.18.4.tar.gz",
    sha256 = "3743a20704efc319070957c45e24ae4626a05ba4b1d6a8961e87520296f1b676",
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

maven_install(
    artifacts = [
        "org.junit.jupiter:junit-jupiter-engine:5.3.2",
        "io.grpc:grpc-testing:1.16.1",
        "io.grpc:grpc-all:1.16.1",
        "org.apache.commons:commons-compress:1.10",
        "javax.inject:javax.inject:1",
        "com.google.code.findbugs:jsr305:3.0.2",
        "com.google.dagger:dagger:2.23.2",
        "org.apache.maven.plugins:maven-compiler-plugin:3.6.1",
        "androidx.annotation:annotation:1.1.0",
        "com.android.support:support-annotations:28.0.0",
        "androidx.core:core:1.0.2",
        "com.android.support.test:runner:1.0.2",
        "androidx.test:runner:1.2.0",
        "androidx.test:rules:1.2.0",
    ],
    repositories = [
        "https://maven.google.com",
        "https://repo1.maven.org/maven2",
        "https://mvnrepository.com",
    ],
)

http_archive(
    name = "com_google_dagger",
    urls = ["https://github.com/google/dagger/archive/dagger-2.23.2.zip"],
    strip_prefix = "dagger-dagger-2.23.2",
    sha256 = "7c3c65bf3e76da985e546804a0eb5eacc394af4b62459f7751c4b261ec9c7770",
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
