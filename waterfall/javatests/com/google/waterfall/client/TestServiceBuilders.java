package com.google.waterfall.client;

import com.google.protobuf.ByteString;
import com.google.waterfall.WaterfallGrpc.WaterfallImplBase;
import com.google.waterfall.WaterfallProto.CmdProgress;
import com.google.waterfall.WaterfallProto.Transfer;
import com.google.waterfall.tar.Tar;
import io.grpc.stub.StreamObserver;
import java.io.IOException;
import java.util.Arrays;
import java.util.List;

public class TestServiceBuilders {
  public static WaterfallImplBase getValidPullServerImpl(String src) throws Exception {
    ByteString.Output output = ByteString.newOutput();
    Tar.tar(src, output);
    final Transfer responseTransfer = Transfer
        .newBuilder()
        .setSuccess(true)
        .setPayload(output.toByteString())
        .build();

    return getValidPullServerImpl(Arrays.asList(responseTransfer));
  }

  public static WaterfallImplBase getValidPullServerImpl(List<Transfer> transfers) {
    return new WaterfallImplBase() {
      @Override
      public void pull(Transfer transfer, StreamObserver<Transfer> responseObserver) {
        for(Transfer t : transfers) {
          responseObserver.onNext(t);
        }
        responseObserver.onCompleted();
      }
    };
  }

  public static WaterfallImplBase getValidPushServerImpl(String dst) throws Exception {
    return new WaterfallImplBase() {
      @Override
      public StreamObserver<Transfer> push(StreamObserver<Transfer> responseObserver) {
        return new StreamObserver<Transfer>() {
          ByteString.Output collected = ByteString.newOutput();

          @Override
          public void onNext(Transfer value) {
            try {
              if (!value.getPayload().isEmpty()) {
                value.getPayload().writeTo(collected);
              }
            } catch(IOException e) {
              throw new RuntimeException(e);
            }
          }

          @Override
          public void onError(Throwable t) {
          }

          @Override
          public void onCompleted() {
            try {
              Tar.untar(collected.toByteString().newInput(), dst);
              responseObserver.onNext(
                  Transfer.newBuilder()
                      .setSuccess(true)
                      .build());

              responseObserver.onCompleted();
            } catch(IOException e) {
              throw new RuntimeException("Exception shouldn't occur.", e);
            }
          }
        };
      }
    };
  }

  public static WaterfallImplBase getValidExecServerimpl(List<CmdProgress> responses) {
    return new WaterfallImplBase() {
      @Override
      public StreamObserver<CmdProgress> exec(StreamObserver<CmdProgress> responseObserver) {
        return new StreamObserver<CmdProgress>() {
          @Override
          public void onNext(CmdProgress value) {
          }

          @Override
          public void onError(Throwable t) {
          }

          @Override
          public void onCompleted() {
            for(CmdProgress cp : responses){
              responseObserver.onNext(cp);
            }
            responseObserver.onCompleted();
          }
        };
      }
    };
  }
}
