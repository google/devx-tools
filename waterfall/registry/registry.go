// Package registry implements the Registry service as very simple/naive KV store
package registry

import (
	"context"
	"fmt"

	waterfall_grpc "github.com/waterfall/proto/waterfall_go_grpc"
)

// Server exposes a kv store service.
type Server struct {
	r map[string]string
}

// Add adds a new device to the registry of devices.
func (s *Server) Add(ctx context.Context, e *waterfall_grpc.Entry) (*waterfall_grpc.OpResult, error) {
	if e.Key == "" {
		return nil, fmt.Errorf("entry key was not specified")
	}

	if e.Val == "" {
		return nil, fmt.Errorf("entry value was not specified")
	}

	s.r[e.Key] = e.Val
	return &waterfall_grpc.OpResult{Status: waterfall_grpc.OpResult_SUCCESS}, nil
}

// Remove removes the entry from the store.
func (s *Server) Remove(ctx context.Context, e *waterfall_grpc.Entry) (*waterfall_grpc.OpResult, error) {
	// Removing the empty string from the map will succeed, however this
	// is most likely an error on the client side.
	if e.Key == "" {
		return nil, fmt.Errorf("entry key was not specified")
	}
	delete(s.r, e.Key)
	return &waterfall_grpc.OpResult{Status: waterfall_grpc.OpResult_SUCCESS}, nil
}

// Get gets the requested key from the service.
func (s *Server) Get(ctx context.Context, e *waterfall_grpc.Entry) (*waterfall_grpc.OpResult, error) {
	val, ok := s.r[e.Key]
	if !ok {
		return &waterfall_grpc.OpResult{Status: waterfall_grpc.OpResult_KEY_NOT_FOUND}, nil
	}
	return &waterfall_grpc.OpResult{
		Status: waterfall_grpc.OpResult_SUCCESS,
		Entry: &waterfall_grpc.Entry{Key: e.Key, Val: val}}, nil
}

// NewServer creates a new empty Server.
func NewServer(ctx context.Context) *Server {
	return NewServerWithEntries(make(map[string]string))
}

// NewServerWithEntries creates a new Server populated with entries.
func NewServerWithEntries(entries map[string]string) *Server {
	return &Server{r: entries}
}
