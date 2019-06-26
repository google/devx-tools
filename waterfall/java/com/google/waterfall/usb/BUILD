exports_files(
    ["AndroidManifest.xml"],
)

filegroup(
    name = "resource_files",
    srcs = glob(["res/**"]),
)

# TODO(mauriciogg): this needs to be proguarded because of dagger
android_binary(
    name = "usb_service",
    manifest = "AndroidManifest.xml",
    resource_files = [":resource_files"],
    deps = [
        ":usb_service_lib",
    ],
)

android_library(
    name = "usb_service_lib",
    srcs = [
        "AndroidUsbModule.java",
        "UsbService.java",
    ],
    manifest = "AndroidManifest.xml",
    resource_files = [":resource_files"],
    deps = [
        ":annotations",
        ":mock_usb_accessory",
        ":mock_usb_manager",
        ":real_usb_accessory",
        ":real_usb_manager",
        ":usb_interfaces",
        "@maven//:javax_inject_javax_inject",
        "@maven//:com_google_code_findbugs_jsr305",
        "@maven//:androidx_annotation_annotation",
        "@maven//:androidx_core_core",
        "@maven//:com_android_support_support_annotations",
        "@com_google_dagger//:dagger_with_compiler",
    ],
)

android_library(
    name = "annotations",
    srcs = [
        "ForService.java",
    ],
    deps = [
        "@maven//:javax_inject_javax_inject",
    ],
)

android_library(
    name = "usb_interfaces",
    srcs = [
        "UsbAccessoryIntf.java",
        "UsbManagerIntf.java",
    ],
)

android_library(
    name = "mock_usb_manager",
    srcs = [
        "MockUsbManager.java",
        "UsbManagerIntf.java",
    ],
    deps = [
        ":mock_usb_accessory",
        ":usb_interfaces",
    ],
)

android_library(
    name = "real_usb_manager",
    srcs = [
        "AoaUsbManager.java",
    ],
    deps = [
        ":real_usb_accessory",
        ":usb_interfaces",
    ],
)

android_library(
    name = "mock_usb_accessory",
    srcs = [
        "MockUsbAccessory.java",
    ],
    deps = [
        ":usb_interfaces",
    ],
)

android_library(
    name = "real_usb_accessory",
    srcs = [
        "AoaUsbAccessory.java",
    ],
    deps = [
        ":usb_interfaces",
    ],
)
