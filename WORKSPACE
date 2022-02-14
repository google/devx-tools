workspace(name = "devx")

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")
load("@bazel_tools//tools/build_defs/repo:git.bzl", "new_git_repository")

# Maven rule for transitive dependencies
RULES_JVM_EXTERNAL_TAG = "1.2"

RULES_JVM_EXTERNAL_SHA = "e5c68b87f750309a79f59c2b69ead5c3221ffa54ff9496306937bfa1c9c8c86b"

http_archive(
    name = "rules_jvm_external",
    sha256 = RULES_JVM_EXTERNAL_SHA,
    strip_prefix = "rules_jvm_external-%s" % RULES_JVM_EXTERNAL_TAG,
    url = "https://github.com/bazelbuild/rules_jvm_external/archive/%s.zip" % RULES_JVM_EXTERNAL_TAG,
)

load("@rules_jvm_external//:defs.bzl", "maven_install")

http_archive(
    name = "com_google_protobuf",
    sha256 = "e8c7601439dbd4489fe5069c33d374804990a56c2f710e00227ee5d8fd650e67",
    strip_prefix = "protobuf-3.11.2",
    urls = ["https://github.com/protocolbuffers/protobuf/archive/v3.11.2.tar.gz"],
)

load("@com_google_protobuf//:protobuf_deps.bzl", "protobuf_deps")

protobuf_deps()

# gRPC Java
# Note that this needs to come before io_bazel_go_rules. Both depend on
# protobuf and the version that io_bazel_rules_go depends on is broken for
# java, so io_grpc_grpc_java needs to get the dep first.
http_archive(
    name = "io_grpc_grpc_java",
    sha256 = "0378bf29029c48ed55f0d2ec210fb539cf1fc76a54da3ce9ecfc458d1ad5cb2b",
    strip_prefix = "grpc-java-1.26.0",
    urls = ["https://github.com/grpc/grpc-java/archive/v1.26.0.tar.gz"],
)

load("@io_grpc_grpc_java//:repositories.bzl", "grpc_java_repositories")

grpc_java_repositories()

http_archive(
    name = "rules_proto",
    sha256 = "602e7161d9195e50246177e7c55b2f39950a9cf7366f74ed5f22fd45750cd208",
    strip_prefix = "rules_proto-97d8af4dc474595af3900dd85cb3a29ad28cc313",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/rules_proto/archive/97d8af4dc474595af3900dd85cb3a29ad28cc313.tar.gz",
        "https://github.com/bazelbuild/rules_proto/archive/97d8af4dc474595af3900dd85cb3a29ad28cc313.tar.gz",
    ],
)

load("@rules_proto//proto:repositories.bzl", "rules_proto_dependencies", "rules_proto_toolchains")

rules_proto_dependencies()

rules_proto_toolchains()

# Go toolchains
http_archive(
    name = "io_bazel_rules_go",
    sha256 = "e88471aea3a3a4f19ec1310a55ba94772d087e9ce46e41ae38ecebe17935de7b",
    urls = [
        "https://storage.googleapis.com/bazel-mirror/github.com/bazelbuild/rules_go/releases/download/v0.20.3/rules_go-v0.20.3.tar.gz",
        "https://github.com/bazelbuild/rules_go/releases/download/v0.20.3/rules_go-v0.20.3.tar.gz",
    ],
)

load(
    "@io_bazel_rules_go//go:deps.bzl",
    "go_register_toolchains",
    "go_rules_dependencies",
)

go_rules_dependencies()

go_register_toolchains()

http_archive(
    name = "bazel_gazelle",
    sha256 = "86c6d481b3f7aedc1d60c1c211c6f76da282ae197c3b3160f54bd3a8f847896f",
    urls = [
        "https://storage.googleapis.com/bazel-mirror/github.com/bazelbuild/bazel-gazelle/releases/download/v0.19.1/bazel-gazelle-v0.19.1.tar.gz",
        "https://github.com/bazelbuild/bazel-gazelle/releases/download/v0.19.1/bazel-gazelle-v0.19.1.tar.gz",
    ],
)

load("@bazel_gazelle//:deps.bzl", "gazelle_dependencies", "go_repository")

gazelle_dependencies()

go_repository(
    name = "org_golang_google_grpc",
    build_file_proto_mode = "disable",
    importpath = "google.golang.org/grpc",
    sum = "h1:zWTV+LMdc3kaiJMSTOFz2UgSBgx8RNQoTGiZu3fR9S0=",
    version = "v1.32.0",

)

go_repository(
    name = "org_golang_x_net",
    importpath = "golang.org/x/net",
    sum = "h1:oWX7TPOiFAMXLq8o0ikBYfCJVlRHBcsciT5bXOrH628=",
    version = "v0.0.0-20190311183353-d8887717615a",
)

go_repository(
    name = "org_golang_x_text",
    importpath = "golang.org/x/text",
    sum = "h1:g61tztE5qeGQ89tm6NTjjM9VPIm088od1l6aSorWRWg=",
    version = "v0.3.0",
)

go_repository(
    name = "org_golang_x_sys",
    commit = "af09f7315aff1cbc48fb21d21aa55d67b4f914c5",
    importpath = "golang.org/x/sys",
)

go_repository(
    name = "org_golang_x_sync",
    commit = "1d60e4601c6fd243af51cc01ddf169918a5407ca",
    importpath = "golang.org/x/sync",
)

go_repository(
    name = "com_github_mdlayher_socket",
    commit = "5540490a708094d62b83da0f0ba6f0457a893b73",
    importpath = "github.com/mdlayher/socket",
)

go_repository(
    name = "com_github_mdlayher_vsock",
    commit = "69ff27ae6148dad0fe15cd2b72e8113c768b7b39",
    importpath = "github.com/mdlayher/vsock",
)

new_git_repository(
    name = "com_github_google_gousb",
    # Use custom BUILD file since we need to specify how to link agains libusb.
    build_file = "@//:BUILD.gousb",
    commit = "64d82086770b8b671e1e7f162372dd37f1f5efba",
    remote = "https://github.com/google/gousb.git",
)

maven_install(
    artifacts = [
        "androidx.annotation:annotation:1.1.0",
        "androidx.core:core:1.0.2",
        "androidx.test:monitor:1.2.0",
        "androidx.test:rules:1.2.0",
        "androidx.test:runner:1.2.0",
        "com.android.support:support-annotations:28.0.0",
        "com.android.support.test:runner:1.0.2",
        "com.google.code.findbugs:jsr305:3.0.2",
        "com.google.dagger:dagger:2.23.2",
        "com.google.dagger:dagger-compiler:2.23.2",
        "javax.inject:javax.inject:1",
        "junit:junit:4.12",
        "org.apache.commons:commons-compress:1.10",
        "org.junit.jupiter:junit-jupiter-engine:5.3.2",
        "org.mockito:mockito-core:2.28.2",
        "org.mockito:mockito-android:2.28.2",
    ],
    repositories = [
        "https://maven.google.com",
        "https://repo1.maven.org/maven2",
        "https://mvnrepository.com",
    ],
)

# Android libs
android_sdk_repository(name = "androidsdk")

http_archive(
    name = "android_test_support",
    sha256 = "01a3a6a88588794b997b46a823157aea06be9bcdc41800b61199893121ef26a3",
    strip_prefix = "android-test-androidx-test-1.2.0",
    urls = ["https://github.com/android/android-test/archive/androidx-test-1.2.0.tar.gz"],
)

load("@android_test_support//:repo.bzl", "android_test_repositories")

android_test_repositories()
