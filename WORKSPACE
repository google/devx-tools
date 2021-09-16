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
    sha256 = "c6003e1d2e7fefa78a3039f19f383b4f3a61e81be8c19356f85b6461998ad3db",
    strip_prefix = "protobuf-3.17.3",
    urls = ["https://github.com/protocolbuffers/protobuf/archive/v3.17.3.tar.gz"],
)

load("@com_google_protobuf//:protobuf_deps.bzl", "protobuf_deps")

protobuf_deps()

# gRPC Java
# Note that this needs to come before io_bazel_go_rules. Both depend on
# protobuf and the version that io_bazel_rules_go depends on is broken for
# java, so io_grpc_grpc_java needs to get the dep first.
http_archive(
    name = "io_grpc_grpc_java",
    sha256 = "340091bf58b05c1a7d4cae5c60d6acde7e82ce24f67d09a16638fe894c0e233f",
    strip_prefix = "grpc-java-1.40.1",
    urls = ["https://github.com/grpc/grpc-java/archive/v1.40.1.tar.gz"],
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
    sha256 = "8e968b5fcea1d2d64071872b12737bbb5514524ee5f0a4f54f5920266c261acb",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/rules_go/releases/download/v0.28.0/rules_go-v0.28.0.zip",
        "https://github.com/bazelbuild/rules_go/releases/download/v0.28.0/rules_go-v0.28.0.zip",
    ],
)

load(
    "@io_bazel_rules_go//go:deps.bzl",
    "go_register_toolchains",
    "go_rules_dependencies",
)

go_rules_dependencies()

go_register_toolchains(version = "1.17")

http_archive(
    name = "bazel_gazelle",
    sha256 = "62ca106be173579c0a167deb23358fdfe71ffa1e4cfdddf5582af26520f1c66f",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/bazel-gazelle/releases/download/v0.23.0/bazel-gazelle-v0.23.0.tar.gz",
        "https://github.com/bazelbuild/bazel-gazelle/releases/download/v0.23.0/bazel-gazelle-v0.23.0.tar.gz",
    ],
)

load("@bazel_gazelle//:deps.bzl", "gazelle_dependencies", "go_repository")

gazelle_dependencies()

go_repository(
    name = "org_golang_google_grpc",
    build_file_proto_mode = "disable",
    importpath = "google.golang.org/grpc",
    sum = "h1:AGJ0Ih4mHjSeibYkFGh1dD9KJ/eOtZ93I6hoHhukQ5Q=",
    version = "v1.40.0",
)

go_repository(
    name = "org_golang_x_net",
    importpath = "golang.org/x/net",
    sum = "h1:E8wdt+zBjoxD3MA65wEc3pl25BsTi7tbkpwc4ANThjc=",
    version = "v0.0.0-20210908191846-a5e095526f91",
)

go_repository(
    name = "org_golang_x_text",
    importpath = "golang.org/x/text",
    sum = "h1:olpwvP2KacW1ZWvsR7uQhoyTYvKAupfQrRGBFM352Gk=",
    version = "v0.3.7",
)

go_repository(
    name = "org_golang_x_sys",
    importpath = "golang.org/x/sys",
    sum = "h1:xrCZDmdtoloIiooiA9q0OQb9r8HejIHYoHGhGCe1pGg=",
    version = "v0.0.0-20210910150752-751e447fb3d0",
)

go_repository(
    name = "org_golang_x_sync",
    importpath = "golang.org/x/sync",
    sum = "h1:5KslGYwFpkhGh+Q16bwMP3cOontH8FOep7tGV86Y7SQ=",
    version = "v0.0.0-20210220032951-036812b2e83c",
)

go_repository(
    name = "com_github_mdlayher_vsock",
    commit = "7ad3638b3fbc5ddf14fc37a1f9d046d1d6dd2013",
    importpath = "github.com/mdlayher/vsock",
    patches = ["//patches:vsock.patch"],
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
