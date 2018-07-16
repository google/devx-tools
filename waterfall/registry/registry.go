// Package registry implements the Registry service as very simple/naive KV store
package registry

import (
	"context"
	"fmt"

	waterfall_grpc "github.com/waterfall/proto/waterfall_go_grpc"
)

type registryServer struct {
	r map[string]string
}

// Add adds a new device to the registry of devices
func (s *registryServer) Add(ctx context.Context, e *waterfall_grpc.Entry) (*waterfall_grpc.OpResult, error) {
	if e.Key == "" {
		return nil, fmt.Errorf("entry key was not specified.")
	}

	if e.Val == "" {
		return nil, fmt.Errorf("entry value was not specified.")
	}

	s.r[e.Key] = e.Val
	return &waterfall_grpc.OpResult{Status: waterfall_grpc.OpResult_SUCCESS}, nil
}

func (s *registryServer) Remove(ctx context.Context, e *waterfall_grpc.Entry) (*waterfall_grpc.OpResult, error) {
	// Removing the empty string from the map will succeed, however this
	// is most likely an error on the client side.
	if e.Key == "" {
		return nil, fmt.Errorf("entry key was not specified.")
	}
	delete(s.r, e.Key)
	return &waterfall_grpc.OpResult{Status: waterfall_grpc.OpResult_SUCCESS}, nil
}

func (s *registryServer) Get(ctx context.Context, e *waterfall_grpc.Entry) (*waterfall_grpc.OpResult, error) {
	val, ok := s.r[e.Key]
	if !ok {
		return &waterfall_grpc.OpResult{Status: waterfall_grpc.OpResult_KEY_NOT_FOUND}, nil
	}
	return &waterfall_grpc.OpResult{
		Status: waterfall_grpc.OpResult_SUCCESS,
		Entry: &waterfall_grpc.Entry{Key: e.Key, Val: val}}, nil
}

// NewRegistryServer creates a new empty registryServer
func NewRegistryServer(ctx context.Context) *registryServer {
	return NewRegistryServerWithEntries(make(map[string]string))
}

// NewRegistryServerWithEntries creates a new registryServer populated with entries
func NewRegistryServerWithEntries(entries map[string]string) *registryServer {
	return &registryServer{r: entries}
}
