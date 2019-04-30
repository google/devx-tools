package server

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const sessionHdr = "x-session-id"

// AuthInterceptor provides Streaming and Unary Server interceptors that provide session ID
// verification for all incoming requests.
type AuthInterceptor struct {
	sessionID string
}

// NewAuthInterceptor provides AuthInterceptor that authorizes requests for given session ID.
func NewAuthInterceptor(sessionID string) *AuthInterceptor {
	return &AuthInterceptor{sessionID: sessionID}
}

func (a *AuthInterceptor) authorize(ctx context.Context) error {
	if a.sessionID != "" {
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return status.Error(codes.Unauthenticated, "missing metadata")
		}
		hdrSessionID, ok := md[sessionHdr]
		if !ok || len(hdrSessionID) != 1 {
			return status.Error(codes.Unauthenticated, "invalid x-session-id header")
		}
		if a.sessionID != sessionHdrVal[0] {
			return status.Error(codes.Unauthenticated, "bad x-session-id value")
		}
	}
	return nil
}

// StreamServerInterceptor provides session ID verification for all streaming server requests.
func (a *AuthInterceptor) StreamServerInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	if err := a.authorize(ss.Context()); err != nil {
		return err
	}
	return handler(srv, ss)
}

// UnaryServerInterceptor provides session ID verification for all Unary server requests.
func (a *AuthInterceptor) UnaryServerInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	if err := a.authorize(ctx); err != nil {
		return nil, err
	}
	return handler(ctx, req)
}
