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

import com.google.common.util.concurrent.ListenableFuture;
import com.google.common.util.concurrent.ListeningExecutorService;
import com.google.common.util.concurrent.MoreExecutors;
import com.google.common.util.concurrent.SettableFuture;
import com.google.protobuf.ByteString;
import com.google.waterfall.WaterfallGrpc;
import com.google.waterfall.WaterfallGrpc.WaterfallStub;
import com.google.waterfall.WaterfallProto.Cmd;
import com.google.waterfall.WaterfallProto.CmdProgress;
import com.google.waterfall.WaterfallProto.Transfer;
import com.google.waterfall.tar.Tar;
import io.grpc.ManagedChannel;
import io.grpc.ManagedChannelBuilder;
import io.grpc.stub.StreamObserver;
import java.io.IOException;
import java.io.OutputStream;
import java.io.PipedInputStream;
import java.io.PipedOutputStream;
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
public final class WaterfallClient {
  private static final int PIPE_BUFFER_SIZE = 256 * 1024;
  private static final int LISTENER_CONCURRENCY = 2;
  private static final int SHUTDOWN_TIMEOUT_SECONDS = 1;

  private final WaterfallStub asyncStub;
  private final ManagedChannel channel;

  /**
   * @param channelBuilder channelBuilder initialized with the server's settings.
   * */
  private WaterfallClient(ManagedChannelBuilder<?> channelBuilder) {
    this.channel = channelBuilder.build();
    asyncStub = WaterfallGrpc.newStub(channel);
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

    /**
     * Returns WaterfallClient with channel initialized.
     */
    public WaterfallClient build() {
      Objects.requireNonNull(
          channelBuilder, "Must specify non-null arg to withChannelBuilder before building.");
      return new WaterfallClient(channelBuilder);
    }
  }

  /** @return executor service with limited concurrency */
  private static ListeningExecutorService newListeners() {
    return MoreExecutors.listeningDecorator(Executors.newFixedThreadPool(LISTENER_CONCURRENCY));
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
    ListeningExecutorService listeners = newListeners();

    try {
      return pullChecked(src, dst, listeners);
    } catch (IOException e) {
      throw new RuntimeException(e);
    } finally {
      listeners.shutdown();
    }
  }

  /**
   * @param src Absolute path to source file on device
   * @param dst Absolute path to destination directory on host using location file system
   * @param listeners Executors used for reading and writing concurrently
   * @throws IOException Thrown by pipes or gRPC errors.
   */
  private ListenableFuture<Void> pullChecked(
      String src, Path dst, ListeningExecutorService listeners) throws IOException {

    PipedInputStream input = new PipedInputStream(PIPE_BUFFER_SIZE);
    PipedOutputStream output = new PipedOutputStream(input);

    Transfer transfer = Transfer.newBuilder().setPath(src).build();
    final SettableFuture<Void> result = SettableFuture.create();

    StreamObserver<Transfer> responseObserver =
        new StreamObserver<Transfer>() {
          @Override
          public void onNext(Transfer value) {
            try {
              value.getPayload().writeTo(output);
            } catch (IOException e) {
              onError(new RuntimeException(e));
            }
          }

          @Override
          public void onError(Throwable t) {
            result.setException(t);
          }

          @Override
          public void onCompleted() {
            try {
              output.close();
            } catch (IOException e) {
              onError(e);
            }
          }
        };

    ListenableFuture<?> unusedUntarFuture = listeners.submit(
        () -> {
          asyncStub.pull(transfer, responseObserver);

          try {
            Tar.untar(input, dst.toString());
            result.set(null);
          } catch (IOException e) {
            result.setException(e);
          } finally {
            try {
              input.close();
            } catch(IOException e) {
              result.setException(e);
            }
          }
        });

    return result;
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
    ListeningExecutorService listeners = newListeners();

    try {
      return pushChecked(src, dst, listeners);
    } catch (IOException e) {
      throw new RuntimeException(e);
    } finally {
      listeners.shutdown();
    }
  }

  /**
   * @param src Absolute path to source file on host using local filesystem.
   * @param dst Absolute path to destination on device
   * @param listeners Executors used for reading and writing concurrently
   * @throws IOException Thrown by pipes or gRPC errors.
   */
  private Future<Void> pushChecked(Path src, String dst, ListeningExecutorService listeners)
      throws IOException{

    final SettableFuture<Void> result = SettableFuture.create();

    StreamObserver<Transfer> responseObserver =  new StreamObserver<Transfer>() {
      @Override
      public void onNext(Transfer transfer) {
        // We don't expect any incoming messages when pushing.
      }

      @Override
      public void onError(Throwable t) {
        result.setException(t);
      }

      @Override
      public void onCompleted() {
        result.set(null);
      }
    };

    StreamObserver<Transfer> requestObserver = asyncStub.push(responseObserver);
    requestObserver.onNext(Transfer.newBuilder().setPath(dst).build());

    PipedInputStream input = new PipedInputStream(PIPE_BUFFER_SIZE);
    PipedOutputStream output = new PipedOutputStream(input);

    ListenableFuture<?> unusedTarFuture =
        listeners.submit(
            () -> {
              try {
                Tar.tar(src.toString(), output);
              } catch(IOException e) {
                result.setException(e);
              } finally {
                try {
                  output.close();
                } catch(IOException ex) {
                  result.setException(ex);
                }
              }
            });

    ListenableFuture<?> unusedTransferFuture =
        listeners.submit(
            () -> {
              try {
                byte[] buff = new byte[PIPE_BUFFER_SIZE];

                while (!result.isDone()) {
                  int r = input.read(buff);

                  if (r == -1) {
                    break;
                  }

                  requestObserver.onNext(
                      Transfer.newBuilder().setPayload(ByteString.copyFrom(buff, 0, r)).build());
                }

                input.close();
                requestObserver.onCompleted();
              } catch (IOException e) {
                requestObserver.onError(e);
                result.setException(e);
              }
            });

    return result;
  }

  /**
   * Executes a command on the device.
   *
   * @param command
   * @param args
   * @param input
   * @param stdout
   * @param stderr
   */
  public ListenableFuture<CmdProgress> exec(
      String command, List<String> args, String input, OutputStream stdout, OutputStream stderr) {

    try {
      return execChecked(command, args, input, stdout, stderr);
    } catch (Exception e) {
      throw new RuntimeException(e);
    }
  }

  /**
   * Executes a command on the device.
   *
   * @param command
   * @param args
   * @param input
   * @param stdout
   * @param stderr
   */
  private ListenableFuture<CmdProgress> execChecked(
      String command, List<String> args, String input, OutputStream stdout, OutputStream stderr) {

    final SettableFuture<CmdProgress> result = SettableFuture.create();

    StreamObserver<CmdProgress> responseObserver = new StreamObserver<CmdProgress>() {
          private CmdProgress last = null;

          @Override
          public void onNext(CmdProgress cmdProgress) {
            try {
              cmdProgress.getStdout().writeTo(stdout);
              cmdProgress.getStderr().writeTo(stderr);
              last = cmdProgress;
            } catch (IOException e) {
              onError(e);
            }
          }

          @Override
          public void onError(Throwable t) {
            result.setException(t);
          }

          @Override
          public void onCompleted() {
            result.set(last);
          }
        };

    StreamObserver<CmdProgress> requestObserver = asyncStub.exec(responseObserver);

    try {
      requestObserver.onNext(
          CmdProgress.newBuilder()
              .setCmd(Cmd.newBuilder().setPath(command).addAllArgs(args).setPipeIn(input != null))
              .build());

      if (input != null) {
        requestObserver.onNext(
            CmdProgress.newBuilder().setStdin(ByteString.copyFromUtf8(input)).build());
      }

      requestObserver.onCompleted();
    } catch (Exception e) {
      requestObserver.onError(e);
      result.setException(e);
    }

    return result;
  }

  /**
   * Cleans up client. Times out if channel termination takes too long.
   *
   * @throws InterruptedException Thrown by channel cleanup.
   * */
  public void shutdown() throws InterruptedException {
    channel.shutdown().awaitTermination(SHUTDOWN_TIMEOUT_SECONDS, TimeUnit.SECONDS);
  }
}
