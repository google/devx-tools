package com.google.waterfall.client;

import static java.nio.charset.StandardCharsets.UTF_8;
import static org.junit.Assert.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.Assert.assertTrue;

import com.google.protobuf.ByteString;
import com.google.waterfall.WaterfallGrpc.WaterfallImplBase;
import com.google.waterfall.WaterfallProto.CmdProgress;
import com.google.waterfall.WaterfallProto.Transfer;
import com.google.waterfall.helpers.FileTestHelper;
import io.grpc.inprocess.InProcessChannelBuilder;
import io.grpc.inprocess.InProcessServerBuilder;
import io.grpc.stub.StreamObserver;
import io.grpc.testing.GrpcCleanupRule;
import io.grpc.util.MutableHandlerRegistry;
import java.io.ByteArrayOutputStream;
import java.io.File;
import java.io.IOException;
import java.io.Writer;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.util.Arrays;
import java.util.concurrent.ExecutionException;
import org.junit.After;
import org.junit.Before;
import org.junit.Rule;
import org.junit.Test;
import org.junit.rules.TemporaryFolder;
import org.junit.runner.RunWith;
import org.junit.runners.JUnit4;

@RunWith(JUnit4.class)
public final class WaterfallClientTest {
  private WaterfallClient client;
  private File pullFile;
  private File pushFile;
  private final MutableHandlerRegistry serviceRegistry = new MutableHandlerRegistry();

  @Rule public TemporaryFolder rootFolder = new TemporaryFolder();

  @Rule public final GrpcCleanupRule grpcCleanup = new GrpcCleanupRule();

  @Before
  public void setUp() throws Exception {
    setupFolders();
    String serverName = InProcessServerBuilder.generateName();

    grpcCleanup.register(
        InProcessServerBuilder.forName(serverName)
            .fallbackHandlerRegistry(serviceRegistry)
            .directExecutor()
            .build()
            .start());

    client =
        WaterfallClient.newBuilder()
            .withChannelBuilder(InProcessChannelBuilder.forName(serverName).directExecutor())
            .build();
  }

  @After
  public void tearDown() throws Exception {
    client.shutdown();
  }

  @Test
  public void testPullFile() throws Exception {
    String src = pullFile.getPath();
    String dst = rootFolder.getRoot().getPath() + "/" + pullFile.getName();

    WaterfallImplBase server = TestServiceBuilders.getValidPullServerImpl(src);
    serviceRegistry.addService(server);

    client.pull(src, dst).get();
    assertTrue(FileTestHelper.fileContentEquals(dst, FileTestHelper.SAMPLE_FILE_CONTENT));
  }

  @Test
  public void testPullDirectory() throws Exception {
    String srcDir = pullFile.getParent();
    String dstDir = rootFolder.getRoot().getPath() + "/pulleddir";
    String dst = dstDir + "/" + pullFile.getName();

    WaterfallImplBase server = TestServiceBuilders.getValidPullServerImpl(srcDir);
    serviceRegistry.addService(server);

    client.pull(srcDir, dstDir).get();
    assertTrue(FileTestHelper.fileContentEquals(dst, FileTestHelper.SAMPLE_FILE_CONTENT));
  }

  @Test
  public void testPullServerThrows() {
    WaterfallImplBase serverWithError =
        new WaterfallImplBase() {
          @Override
          public void pull(Transfer transfer, StreamObserver<Transfer> responseObserver) {
            responseObserver.onError(new IOException("Server is throwing an error."));
          }
        };

    serviceRegistry.addService(serverWithError);

    String dst = rootFolder.getRoot().getPath() + "/file_does_not_exist.txt";
    assertThrows(ExecutionException.class, () -> client.pull(pullFile.getPath(), dst).get());
  }

  @Test
  public void testPushFile() throws Exception {
    String src = pushFile.getPath();
    String dst = rootFolder.getRoot() + "/" + pushFile.getName();

    WaterfallImplBase server = TestServiceBuilders.getValidPushServerImpl(dst);
    serviceRegistry.addService(server);

    client.push(src, dst).get();
    assertTrue(FileTestHelper.fileContentEquals(dst, FileTestHelper.SAMPLE_FILE_CONTENT));
  }

  @Test
  public void testPushDirectory() throws Exception {
    String srcDir = pushFile.getParent();
    String dstDir = rootFolder.getRoot() + "/pulleddir";
    String dst = dstDir + "/" + pushFile.getName();

    WaterfallImplBase server = TestServiceBuilders.getValidPushServerImpl(dstDir);
    serviceRegistry.addService(server);

    client.push(srcDir, dstDir).get();
    assertTrue(FileTestHelper.fileContentEquals(dst, FileTestHelper.SAMPLE_FILE_CONTENT));
  }

  @Test
  public void testPushServerThrows() {
    String src = pushFile.getPath();
    String dst = rootFolder.getRoot().getPath() + "/" + pushFile.getName();

    WaterfallImplBase serverWithError =
        new WaterfallImplBase() {
          @Override
          public StreamObserver<Transfer> push(StreamObserver<Transfer> responseObserver) {
            return new StreamObserver<Transfer>() {
              @Override
              public void onNext(Transfer value) {
                responseObserver.onError(new IOException("Server is throwing an error."));
              }

              @Override
              public void onError(Throwable t) {}

              @Override
              public void onCompleted() {}
            };
          }
        };
    serviceRegistry.addService(serverWithError);

    assertThrows(ExecutionException.class, () -> client.push(src, dst).get());
  }

  @Test
  public void testExecStdOut() throws Exception {
    ByteArrayOutputStream stdout = new ByteArrayOutputStream();
    ByteArrayOutputStream stderr = new ByteArrayOutputStream();
    String expectedResults = "FakeStdoutResults";

    WaterfallImplBase server =
        TestServiceBuilders.getValidExecServerimpl(
            Arrays.asList(
                CmdProgress.newBuilder()
                    .setStdout(ByteString.copyFromUtf8(expectedResults))
                    .setExitCode(0)
                    .build()));
    serviceRegistry.addService(server);

    client.exec("/bin/ls", Arrays.asList("."), null, stdout, stderr).get();
    assertEquals(expectedResults, stdout.toString(StandardCharsets.UTF_8.toString()));
  }

  @Test
  public void testExecStdError() throws Exception {
    ByteArrayOutputStream stdout = new ByteArrayOutputStream();
    ByteArrayOutputStream stderr = new ByteArrayOutputStream();
    String expectedResults = "FakeStderrResults";

    WaterfallImplBase server =
        TestServiceBuilders.getValidExecServerimpl(
            Arrays.asList(
                CmdProgress.newBuilder()
                    .setStderr(ByteString.copyFromUtf8(expectedResults))
                    .setExitCode(1)
                    .build()));
    serviceRegistry.addService(server);

    client.exec("/bin/ls", Arrays.asList("."), null, stdout, stderr).get();
    assertEquals(expectedResults, stderr.toString(StandardCharsets.UTF_8.toString()));
  }

  @Test
  public void testExecServerThrows() {
    ByteArrayOutputStream stdout = new ByteArrayOutputStream();
    ByteArrayOutputStream stderr = new ByteArrayOutputStream();

    WaterfallImplBase server =
        new WaterfallImplBase() {
          @Override
          public StreamObserver<CmdProgress> exec(StreamObserver<CmdProgress> responseObserver) {
            responseObserver.onError(new RuntimeException("This exception should be returned"));
            return new StreamObserver<CmdProgress>() {
              @Override
              public void onNext(CmdProgress value) {}

              @Override
              public void onError(Throwable t) {}

              @Override
              public void onCompleted() {
                responseObserver.onCompleted();
              }
            };
          }
        };
    serviceRegistry.addService(server);

    assertThrows(
        ExecutionException.class,
        () -> client.exec("/bin/ls", Arrays.asList("."), null, stdout, stderr).get());
  }

  public void setupFolders() throws Exception {
    File pushdir = rootFolder.newFolder("pushdir");
    pushdir.mkdir();
    pushFile = new File(pushdir, "push.txt");

    try(Writer pushWriter = Files.newBufferedWriter(pushFile.toPath(), UTF_8)) {
      pushWriter.write(FileTestHelper.SAMPLE_FILE_CONTENT);
    }

    File pulldir = rootFolder.newFolder("pulldir");
    pulldir.mkdir();
    pullFile = new File(pulldir, "pull.txt");

    try(Writer pullWriter = Files.newBufferedWriter(pullFile.toPath(), UTF_8)) {
      pullWriter.write(FileTestHelper.SAMPLE_FILE_CONTENT);
    }
  }
}
