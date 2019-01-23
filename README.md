# H2O project

H2O allow controlling Android devices via a gRPC service.
Currently, only emulators are supported natively. For
physical devices, H2O falls back to ADB.

A Go client is implemented under waterfall/client/ and
a ADB command line compatible binary can be found under
waterfall/client/adb/.

This is not an officially supported Google product.
