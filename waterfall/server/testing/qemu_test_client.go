// Package testing provides functionality to test a server backed by different connections
package testing

import (
	"bytes"
	"context"
	"io"

	waterfall_grpc "github.com/waterfall/proto/waterfall_go_grpc"
	"golang.org/x/sync/errgroup"
)

func Echo(ctx context.Context, client waterfall_grpc.WaterfallClient, r []byte) ([]byte, error) {
	stream, err := client.Echo(ctx)
	if err != nil {
		return nil, err
	}
	eg, ctx := errgroup.WithContext(ctx)
	rec := new(bytes.Buffer)
	eg.Go(func() error {
		for {
			in, err := stream.Recv()
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return err
			}
			rec.Write(in.Payload)
		}
	})
	eg.Go(func() error {
		send := bytes.NewBuffer(r)
		b := make([]byte, 32*1024)
		for {
			n, err := send.Read(b)
			if n > 0 {
				p := &waterfall_grpc.Message{Payload: b[0:n]}
				if err := stream.Send(p); err != nil {
					return err
				}
			}
			if err == io.EOF {
				break
			}
			if err != nil {
				return err
			}
		}
		return stream.CloseSend()
	})
	if err := eg.Wait(); err != nil {
		return nil, err
	}
	return rec.Bytes(), nil
}
