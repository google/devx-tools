# H2O project

H2O allows controlling Android devices via a gRPC interface.

### Using the tool with the emulator

In order to use H2O, the emulator needs to be started with
[qemu pipes](https://android.googlesource.com/platform/external/qemu/+/master/docs/ANDROID-QEMU-PIPE.TXT)
enabled.

`emulator @${AVD} -unix-pipe sockets/h2o` will start the emulator allowing
outbound connections to an abstract unix domain socket in the emulator working
dir.

Start the server on the device. By default it will try to connect to
sockets/h2o, but this can be configured via the addr flag.

```
bazel build //waterfall/server:server_bin_386

# In the device:
./sever_bin_386 [--addr=<qemu|tcp|unix>:addr]

```

Start the forwarding server. Thhis servers forwards the connection from
the host to the device through the qemu pipe.

```
bazel build waterfall/forward:forward_bin

./forward_bin --listen_addr <tcp|unix>:addr --connect_addr <qemu|tcp|unix>:addr

```

The client can start a regular gRPC connection to --listen_addr.

### Using the tool with physical devices

At the moment, H2O has no USB support (though this is a planned feature).
In order to work with physical devices, the server needs to be started
on the device listening on a normal TCP or Unix address and forwarder through
ADB.

`adb forward tcp:${server_port} tcp:${server_port}`

# Command line client

A client compatible with ADB command line is available under waterfall/client/adb.
This client can be used as a drop in replacement for ADB.

**This is not an officially supported Google product.**
