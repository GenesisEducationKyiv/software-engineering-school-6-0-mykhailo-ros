package grpc

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const internalTokenMetadataKey = "x-internal-token"

// AuthUnaryInterceptor mirrors handler.RequireInternalToken: if token is empty
// the check is skipped (local dev mode), otherwise every unary call must carry
// a matching x-internal-token metadata entry.
func AuthUnaryInterceptor(token string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if token == "" {
			return handler(ctx, req)
		}

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing internal token")
		}
		values := md.Get(internalTokenMetadataKey)
		if len(values) == 0 || values[0] != token {
			return nil, status.Error(codes.Unauthenticated, "invalid or missing internal token")
		}
		return handler(ctx, req)
	}
}
