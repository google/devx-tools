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

import com.google.common.util.concurrent.FutureCallback;
import com.google.common.util.concurrent.Futures;
import com.google.common.util.concurrent.ListenableFuture;
import com.google.common.util.concurrent.ListeningExecutorService;
import com.google.common.util.concurrent.MoreExecutors;
import com.google.common.util.concurrent.SettableFuture;
import com.google.protobuf.ByteString;
import com.google.waterfall.WaterfallGrpc.WaterfallStub;
import com.google.waterfall.WaterfallProto.Cmd;
import com.google.waterfall.WaterfallProto.CmdProgress;
import com.google.waterfall.WaterfallProto.Transfer;
import com.google.waterfall.tar.Tar;
import io.grpc.stub.StreamObserver;
import java.io.IOException;
import java.io.InputStream;
import java.io.OutputStream;
import java.io.PipedInputStream;
import java.io.PipedOutputStream;
import java.nio.file.Path;
import java.util.List;
import java.util.concurrent.Future;

/**
 * A static client for executing commands on Waterfall, that provides calling users full control on
 * resources such as GRPC stubs and Executor services, instead of the client instantiating one for
 * them.
 */
public class StaticWaterfallClient {

  private static final int PIPE_BUFFER_SIZE = 256 * 1024;

  /**
   * Pulls the specified file/dir from src into dst.
   *
   * @param asyncStub The waterfall GRPC stub on which to send pull request
   * @param executorService An executor service on which to perform async operations.
   * @param src Absolute path to source file on device
   * @param dst Absolute path to destination directory on host using location file system
   */
  public static ListenableFuture<Void> pull(
      WaterfallStub asyncStub, ListeningExecutorService executorService, String src, Path dst) {
    try {
      PipedInputStream input = new PipedInputStream(PIPE_BUFFER_SIZE);
      PipedOutputStream output = new PipedOutputStream(input);
      final SettableFuture<Void> future = SettableFuture.create();
      pullFromWaterfall(asyncStub, src, output, future);
      final ListenableFuture<Void> untarFuture =
          executorService.submit(
              () -> {
                try {
                  Tar.untar(input, dst.toString());
                  future.set(null);
                } catch (IOException e) {
                  future.setException(e);
                } finally {
                  try {
                    output.close();
                  } catch (IOException e) {
                    future.setException(e);
                  }
                }
                return null;
              });
      // Cancel running untar if there was an exception in pulling file from waterfall.
      Futures.addCallback(
          future,
          new FutureCallback<Void>() {
            @Override
            public void onSuccess(Void result) {}

            @Override
            public void onFailure(Throwable t) {
              untarFuture.cancel(true);
            }
          },
          MoreExecutors.directExecutor());
      return future;
    } catch (IOException e) {
      throw new WaterfallRuntimeException("Unable to pull src files/dirs from device.", e);
    }
  }

  /**
   * Pulls the specified file from src into output stream. Only a single src file is accepted
   *
   * @param asyncStub The waterfall GRPC stub on which to send pull request
   * @param executorService An executor service on which to perform async operations.
   * @param src Absolute path to source file on device. This should point to an existing file on the
   *     device, that is not a symlink or a directory.
   * @param out Output stream where the contents of src files will be written to.
   */
  public static ListenableFuture<Void> pullFile(
      WaterfallStub asyncStub,
      ListeningExecutorService executorService,
      String src,
      OutputStream out) {
    try {
      PipedInputStream input = new PipedInputStream(PIPE_BUFFER_SIZE);
      PipedOutputStream output = new PipedOutputStream(input);
      final SettableFuture<Void> future = SettableFuture.create();
      pullFromWaterfall(asyncStub, src, output, future);
      final ListenableFuture<Void> untarFuture =
          executorService.submit(
              () -> {
                try {
                  Tar.untarFile(input, out);
                  future.set(null);
                } catch (IOException e) {
                  future.setException(e);
                } finally {
                  try {
                    output.close();
                  } catch (IOException e) {
                    future.setException(e);
                  }
                }
                return null;
              });
      // Cancel running untar if there was an exception in pulling file from waterfall.
      Futures.addCallback(
          future,
          new FutureCallback<Void>() {
            @Override
            public void onSuccess(Void result) {}

            @Override
            public void onFailure(Throwable t) {
              untarFuture.cancel(true);
            }
          },
          MoreExecutors.directExecutor());
      return future;
    } catch (IOException e) {
      throw new WaterfallRuntimeException("Unable to pull src file from device.", e);
    }
  }

  private static void pullFromWaterfall(
      WaterfallStub asyncStub, String src, OutputStream output, SettableFuture<Void> future) {
    Transfer transfer = Transfer.newBuilder().setPath(src).build();
    StreamObserver<Transfer> responseObserver =
        new StreamObserver<Transfer>() {
          @Override
          public void onNext(Transfer value) {
            try {
              value.getPayload().writeTo(output);
            } catch (IOException e) {
              onError(new WaterfallRuntimeException("Unable to pull file(s) from device.", e));
            }
          }

          @Override
          public void onError(Throwable t) {
            future.setException(t);
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
    asyncStub.pull(transfer, responseObserver);
  }

  /**
   * Push the specified file/dir from src into dst.
   *
   * @param asyncStub The waterfall GRPC stub on which to send push request
   * @param executorService An executor service on which to perform async operations
   * @param src Absolute path to source file on host using local filesystem
   * @param dst Absolute path to destination on device
   */
  public static Future<Void> push(
      WaterfallStub asyncStub, ListeningExecutorService executorService, Path src, String dst) {
    try {
      PipedInputStream input = new PipedInputStream(PIPE_BUFFER_SIZE);
      PipedOutputStream output = new PipedOutputStream(input);
      final SettableFuture<Void> future = SettableFuture.create();
      ListenableFuture<Void> unusedTarFuture =
          executorService.submit(
              () -> {
                try {
                  Tar.tar(src.toString(), output);
                } catch (IOException e) {
                  future.setException(e);
                } finally {
                  try {
                    output.close();
                  } catch (IOException e) {
                    future.setException(e);
                  }
                }
                return null;
              });
      pushToWaterfall(asyncStub, executorService, input, dst, future);
      return future;
    } catch (IOException e) {
      throw new WaterfallRuntimeException("Unable to push file(s) into device", e);
    }
  }

  /**
   * Push a byte array into destination file onto device.
   *
   * @param asyncStub The waterfall GRPC stub on which to send push request
   * @param executorService An executor service on which to perform async operations
   * @param src byte array of a single file content to be transferred to device.
   * @param dst Absolute path to destination on device
   */
  public static Future<Void> pushBytes(
      WaterfallStub asyncStub,
      ListeningExecutorService executorService,
      byte[] src,
      String dst) {
    try {
      PipedInputStream input = new PipedInputStream(PIPE_BUFFER_SIZE);
      PipedOutputStream output = new PipedOutputStream(input);
      final SettableFuture<Void> future = SettableFuture.create();
      ListenableFuture<Void> unusedTarFuture =
          executorService.submit(
              () -> {
                try {
                  Tar.tarFile(src, output);
                } catch (IOException e) {
                  future.setException(e);
                } finally {
                  try {
                    output.close();
                  } catch (IOException e) {
                    future.setException(e);
                  }
                }
                return null;
              });
      pushToWaterfall(asyncStub, executorService, input, dst, future);
      return future;
    } catch (IOException e) {
      throw new WaterfallRuntimeException("Unable to push bytes into device", e);
    }
  }

  private static void pushToWaterfall(
      WaterfallStub asyncStub,
      ListeningExecutorService executorService,
      InputStream in,
      String dst,
      SettableFuture<Void> future) {
    StreamObserver<Transfer> responseObserver =
        new StreamObserver<Transfer>() {
          @Override
          public void onNext(Transfer transfer) {
            // We don't expect any incoming messages when pushing.
          }

          @Override
          public void onError(Throwable t) {
            future.setException(t);
          }

          @Override
          public void onCompleted() {
            future.set(null);
          }
        };

    StreamObserver<Transfer> requestObserver = asyncStub.push(responseObserver);
    requestObserver.onNext(Transfer.newBuilder().setPath(dst).build());

    final ListenableFuture<?> transferFuture =
        executorService.submit(
            () -> {
              try {
                byte[] buff = new byte[PIPE_BUFFER_SIZE];

                while (!future.isDone()) {
                  int r = in.read(buff);

                  if (r == -1) {
                    break;
                  }

                  requestObserver.onNext(
                      Transfer.newBuilder().setPayload(ByteString.copyFrom(buff, 0, r)).build());
                }

                in.close();
                requestObserver.onCompleted();
              } catch (IOException e) {
                requestObserver.onError(e);
                future.setException(e);
              }
            });
    Futures.addCallback(
        future,
        new FutureCallback<Void>() {
          @Override
          public void onSuccess(Void result) {}

          @Override
          public void onFailure(Throwable t) {
            transferFuture.cancel(true);
          }
        },
        MoreExecutors.directExecutor());
  }

  /**
   * Executes a command on the device.
   *
   * @param asyncStub The Waterfall stub on which to send command request
   * @param command executable command on device
   * @param args args list for executable command on device
   * @param input stdin input for executable command on device
   * @param stdout captures any standard output from executing command on device.
   * @param stderr captures any standard error from executing command on device.
   */
  public static ListenableFuture<CmdProgress> exec(
      WaterfallStub asyncStub,
      String command,
      List<String> args,
      String input,
      OutputStream stdout,
      OutputStream stderr) {

    try {
      return execChecked(asyncStub, command, args, input, stdout, stderr);
    } catch (Exception e) {
      throw new WaterfallRuntimeException("Exception running waterfall exec command", e);
    }
  }

  private static ListenableFuture<CmdProgress> execChecked(
      WaterfallStub asyncStub,
      String command,
      List<String> args,
      String input,
      OutputStream stdout,
      OutputStream stderr) {

    final SettableFuture<CmdProgress> result = SettableFuture.create();

    StreamObserver<CmdProgress> responseObserver =
        new StreamObserver<CmdProgress>() {
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

  /** Generic runtime exception thrown by this client. */
  public static final class WaterfallRuntimeException extends RuntimeException {
    WaterfallRuntimeException(String msg, Throwable cause) {
      super(msg, cause);
    }
  }
}
