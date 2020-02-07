// Copyright 2019 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Note that the Go client is the reference client implementation for the waterfall service

package com.google.waterfall.client;

import com.google.common.base.Preconditions;
import com.google.common.util.concurrent.ListenableFuture;
import com.google.common.util.concurrent.ListeningExecutorService;
import com.google.common.util.concurrent.MoreExecutors;
import com.google.waterfall.WaterfallGrpc;
import com.google.waterfall.WaterfallGrpc.WaterfallStub;
import com.google.waterfall.WaterfallProto.CmdProgress;
import io.grpc.ManagedChannel;
import io.grpc.ManagedChannelBuilder;
import java.io.OutputStream;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.util.List;
import java.util.Objects;
import java.util.concurrent.Executors;
import java.util.concurrent.Future;
import java.util.concurrent.TimeUnit;

/**
 * Client for the waterfall service using gRPC. Executes asynchronously.
 * */
public class WaterfallClient {
  private static final int SHUTDOWN_TIMEOUT_SECONDS = 1;

  private final WaterfallStub asyncStub;
  private final ManagedChannel channel;
  private final ListeningExecutorService executorService;
  private final boolean shouldCleanupExecutorService;

  /** @param channelBuilder channelBuilder initialized with the server's settings. */
  private WaterfallClient(
      ManagedChannelBuilder<?> channelBuilder,
      ListeningExecutorService executorService,
      boolean shouldCleanupExecutorService) {
    this.channel = channelBuilder.build();
    asyncStub = WaterfallGrpc.newStub(channel);
    this.executorService = executorService;
    this.shouldCleanupExecutorService = shouldCleanupExecutorService;
  }

  /**
   * Factory for WaterfallClient builders.
   *
   * @return A new WaterfallClient builder
   */
  public static Builder newBuilder() {
    return new Builder();
  }

  /** Builder for WaterfallClient. */
  public static class Builder {
    private ManagedChannelBuilder<?> channelBuilder;
    private ListeningExecutorService executorService;
    private boolean shouldCleanupExecutorService = true;

    /**
     * Returns same builder caller.
     *
     * @param channelBuilder channelBuilder initialized with the server's settings.
     */
    public Builder withChannelBuilder(ManagedChannelBuilder<?> channelBuilder) {
      // Don't realize the channel just yet. Wait until instance creation so we can safely pass the
      // builder object in instances where the server is not running. This is mostly useful
      // for Guice.
      this.channelBuilder = channelBuilder;
      return this;
    }

    public Builder withListeningExecutorService(ListeningExecutorService executorService) {
      Preconditions.checkArgument(!executorService.isShutdown());
      this.executorService = executorService;
      shouldCleanupExecutorService = false;
      return this;
    }

    /**
     * Returns WaterfallClient with channel initialized.
     */
    public WaterfallClient build() {
      Objects.requireNonNull(
          channelBuilder, "Must specify non-null arg to withChannelBuilder before building.");
      if (this.executorService == null) {
        this.executorService = MoreExecutors.listeningDecorator(Executors.newCachedThreadPool());
      }
      return new WaterfallClient(channelBuilder, executorService, shouldCleanupExecutorService);
    }
  }

  /**
   * Pulls the specified file/dir from src into dst.
   *
   * @param src Absolute path to source file on device
   * @param dst Absolute path to destination directory on host using location file system
   */
  public ListenableFuture<Void> pull(String src, String dst) {
    return this.pull(src, Paths.get(dst));
  }

  /**
   * Pulls the specified file/dir from src into dst.
   *
   * @param src Absolute path to source file on device
   * @param dst Absolute path to destination directory on host using location file system
   */
  public ListenableFuture<Void> pull(String src, Path dst) {
    return StaticWaterfallClient.pull(asyncStub, executorService, src, dst);
  }

  /**
   * Pulls the specified file from src into output stream. Only a single src file is accepted
   *
   * @param src Absolute path to source file on device. This should point to an existing file on the
   *     device, that is not a symlink or a directory.
   * @param out Output stream where the contents of src files will be written to.
   */
  public ListenableFuture<Void> pullFile(String src, OutputStream out) {
    return StaticWaterfallClient.pullFile(asyncStub, executorService, src, out);
  }

  /**
   * Push the specified file/dir from src into dst.
   *
   * @param src Absolute path to source file on host using local filesystem
   * @param dst Absolute path to destination on device
   */
  public Future<Void> push(String src, String dst) {
    return this.push(Paths.get(src), dst);
  }

  /**
   * Push the specified file/dir from src into dst.
   *
   * @param src Absolute path to source file on host using local filesystem.
   * @param dst Absolute path to destination on device
   */
  public Future<Void> push(Path src, String dst) {
    return StaticWaterfallClient.push(asyncStub, executorService, src, dst);
  }

  /**
   * Push a byte array into destination file onto device.
   *
   * @param src byte array of a single file content to be transferred to device.
   * @param dst Absolute path to destination on device
   */
  public Future<Void> pushBytes(byte[] src, String dst) {
    return StaticWaterfallClient.pushBytes(asyncStub, executorService, src, dst);
  }

  /**
   * Executes a command on the device.
   *
   * @param command executable command on device
   * @param args args list for executable command on device
   * @param input stdin input for executable command on device
   * @param stdout captures any standard output from executing command on device.
   * @param stderr captures any standard error from executing command on device.
   */
  public ListenableFuture<CmdProgress> exec(
      String command, List<String> args, String input, OutputStream stdout, OutputStream stderr) {
    return StaticWaterfallClient.exec(asyncStub, command, args, input, stdout, stderr);
  }

  /**
   * Cleans up client. Times out if channel termination takes too long.
   *
   * @throws InterruptedException Thrown by channel cleanup.
   */
  public void shutdown() throws InterruptedException {
    channel.shutdown().awaitTermination(SHUTDOWN_TIMEOUT_SECONDS, TimeUnit.SECONDS);
    if (shouldCleanupExecutorService) {
      executorService.shutdown();
    }
  }
}
