package client

import (
	"context"
	"fmt"
	"time"

	subscriptionv1 "github-release-notifier/gen/subscription/v1"

	"notification-service/internal/domain"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

const grpcCallTimeout = 10 * time.Second

const internalTokenMetadataKey = "x-internal-token"

type GRPCClient struct {
	conn   *grpc.ClientConn
	client subscriptionv1.SubscriptionServiceClient
}

func NewGRPCClient(addr, internalToken string) (*GRPCClient, error) {
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(tokenUnaryInterceptor(internalToken)),
	)
	if err != nil {
		return nil, fmt.Errorf("grpc client: dial: %w", err)
	}
	return &GRPCClient{conn: conn, client: subscriptionv1.NewSubscriptionServiceClient(conn)}, nil
}

func tokenUnaryInterceptor(token string) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		ctx = metadata.AppendToOutgoingContext(ctx, internalTokenMetadataKey, token)
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

func (c *GRPCClient) FindAllConfirmed(ctx context.Context) ([]domain.Subscription, error) {
	ctx, cancel := context.WithTimeout(ctx, grpcCallTimeout)
	defer cancel()

	resp, err := c.client.ListConfirmedSubscriptions(ctx, &subscriptionv1.ListConfirmedSubscriptionsRequest{})
	if err != nil {
		return nil, fmt.Errorf("grpc client: list confirmed: %w", err)
	}

	subs := make([]domain.Subscription, len(resp.GetSubscriptions()))
	for i, s := range resp.GetSubscriptions() {
		subs[i] = domain.Subscription{
			ID:          int(s.GetId()),
			Email:       s.GetEmail(),
			Repo:        s.GetRepo(),
			LastSeenTag: s.GetLastSeenTag(),
		}
	}
	return subs, nil
}

func (c *GRPCClient) UpdateLastSeenTag(ctx context.Context, id int, tag string) error {
	ctx, cancel := context.WithTimeout(ctx, grpcCallTimeout)
	defer cancel()

	_, err := c.client.UpdateLastSeenTag(ctx, &subscriptionv1.UpdateLastSeenTagRequest{
		Id:          int64(id),
		LastSeenTag: tag,
	})
	if err != nil {
		return fmt.Errorf("grpc client: update tag: %w", err)
	}
	return nil
}

func (c *GRPCClient) Close() error {
	return c.conn.Close()
}
