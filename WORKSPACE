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
    sha256 = "3bd7828aa5af4b13b99c191e8b1e884ebfa9ad371b0ce264605d347f135d2568",
    strip_prefix = "protobuf-3.19.4",
    urls = [
        "https://mirror.bazel.build/github.com/protocolbuffers/protobuf/archive/v3.19.4.tar.gz",
        "https://github.com/protocolbuffers/protobuf/archive/v3.19.4.tar.gz",
    ],
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
    sha256 = "91585017debb61982f7054c9688857a2ad1fd823fc3f9cb05048b0025c47d023",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/rules_go/releases/download/v0.42.0/rules_go-v0.42.0.zip",
        "https://github.com/bazelbuild/rules_go/releases/download/v0.42.0/rules_go-v0.42.0.zip",
    ],
)

http_archive(
    name = "bazel_gazelle",
    sha256 = "d3fa66a39028e97d76f9e2db8f1b0c11c099e8e01bf363a923074784e451f809",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/bazel-gazelle/releases/download/v0.33.0/bazel-gazelle-v0.33.0.tar.gz",
        "https://github.com/bazelbuild/bazel-gazelle/releases/download/v0.33.0/bazel-gazelle-v0.33.0.tar.gz",
    ],
)

load("@bazel_gazelle//:deps.bzl", "gazelle_dependencies", "go_repository")

go_repository(
    name = "org_golang_google_protobuf",
    importpath = "google.golang.org/protobuf",
    sum = "h1:w43yiav+6bVFTBQFZX0r7ipe9JQ1QsbMgHwbBziscLw=",
    version = "v1.28.0",
)

go_repository(
    name = "org_golang_google_grpc",
    build_file_proto_mode = "disable",
    importpath = "google.golang.org/grpc",
    sum = "h1:kd48UiU7EHsV4rnLyOJRuP/Il/UHE7gdDAQ+SZI7nZk=",
    version = "v1.52.0",
)

go_repository(
    name = "org_golang_x_net",
    importpath = "golang.org/x/net",
    sum = "h1:lUkvobShwKsOesNfWWlCS5q7fnbG1MEliIzwu886fn8=",
    version = "v0.0.0-20220526153639-5463443f8c37",
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
    sum = "h1:dGzPydgVsqGcTRVwiLJ1jVbufYwmzD3LfVPLKsKg+0k=",
    version = "v0.0.0-20220520151302-bc2c85ada10a",
)

go_repository(
    name = "org_golang_x_sync",
    importpath = "golang.org/x/sync",
    sum = "h1:w8s32wxx3sY+OjLlv9qltkLU5yvJzxjjgiHWLjdIcw4=",
    version = "v0.0.0-20220513210516-0976fa681c29",
)

go_repository(
    name = "org_golang_x_mod",
    importpath = "golang.org/x/mod",
    sum = "h1:OJxoQ/rynoF0dcCdI7cLPktw/hR2cueqYfjm43oqK38=",
    version = "v0.5.1",
)

go_repository(
    name = "org_golang_x_xerrors",
    importpath = "golang.org/x/xerrors",
    sum = "h1:5Pf6pFKu98ODmgnpvkJ3kFUOQGGLIzLIkbzUHp47618=",
    version = "v0.0.0-20220517211312-f3a8303e98df",
)

go_repository(
    name = "com_github_mdlayher_socket",
    importpath = "github.com/mdlayher/socket",
    sum = "h1:XZA2X2TjdOwNoNPVPclRCURoX/hokBY8nkTmRZFEheM=",
    version = "v0.2.3",
)

go_repository(
    name = "com_github_mdlayher_vsock",
    importpath = "github.com/mdlayher/vsock",
    sum = "h1:8lFuiXQnmICBrCIIA9PMgVSke6Fg6V4+r0v7r55k88I=",
    version = "v1.1.1",
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
android_sdk_repository(
    name = "androidsdk",
)

android_ndk_repository(
    name = "androidndk",
    api_level = 21
)
register_toolchains("@androidndk//:all")

http_archive(
    name = "android_test_support",
    sha256 = "01a3a6a88588794b997b46a823157aea06be9bcdc41800b61199893121ef26a3",
    strip_prefix = "android-test-androidx-test-1.2.0",
    urls = ["https://github.com/android/android-test/archive/androidx-test-1.2.0.tar.gz"],
)

load("@android_test_support//:repo.bzl", "android_test_repositories")

android_test_repositories()

load(
    "@io_bazel_rules_go//go:deps.bzl",
    "go_register_toolchains",
    "go_rules_dependencies",
)

go_rules_dependencies()

go_register_toolchains(version = "1.20.7")

gazelle_dependencies()
