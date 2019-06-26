load("//tools/build_defs/android:rules.bzl", "android_binary", "android_instrumentation_test", "android_library")

android_instrumentation_test(
    name = "usb_service_test",
    size = "small",
    target_device = "@android_test_support//tools/android/emulated_devices/generic_phone:google_21_x86",
    test_app = ":test_binary",
)

android_library(
    name = "test_lib",
    testonly = 1,
    srcs = glob(
        ["**/*.java"],
    ),
    manifest = "AndroidManifest.xml",
    deps = [
        "//java/com/google/waterfall/usb:mock_usb_manager",
        "//java/com/google/waterfall/usb:usb_interfaces",
        "//java/com/google/waterfall/usb:usb_service_lib",
        "@maven//:androidx_test_runner",
        "@maven//:androidx_test_rules",
        "@maven//:com_google_dagger_dagger",
        "@maven//:com_google_dagger_dagger_android",
        "@maven//:javax_inject_javax_inject",
        "@com_google_dagger//:dagger_with_compiler",
    ],
)

android_binary(
    name = "test_binary",
    testonly = 1,
    instruments = "//java/com/google/waterfall/usb:usb_service",
    manifest = "AndroidManifest.xml",
    deps = [":test_lib"],
)
