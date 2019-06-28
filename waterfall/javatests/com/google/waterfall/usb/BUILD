
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
        "//waterfall/java/com/google/waterfall/usb:mock_usb_manager",
        "//waterfall/java/com/google/waterfall/usb:usb_interfaces",
        "//waterfall/java/com/google/waterfall/usb:usb_service_lib",
        "@maven//:androidx_test_rules",
        "@maven//:androidx_test_monitor",
        "@maven//:androidx_test_runner",
        "@maven//:javax_inject_javax_inject",
        "@maven//:org_mockito_mockito_core",
        "@maven//:org_mockito_mockito_android",
        "@maven//:junit_junit",
    ],
)

android_binary(
    name = "test_binary",
    testonly = 1,
    instruments = "//waterfall/java/com/google/waterfall/usb:usb_service",
    manifest = "AndroidManifest.xml",
    deps = [":test_lib"],
)
