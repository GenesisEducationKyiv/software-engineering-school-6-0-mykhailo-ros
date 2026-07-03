package client

import (
	"context"

	"notification-service/internal/domain"
)

// SubscriptionClient is satisfied by both RESTClient and GRPCClient, letting
// the scheduler talk to subscription-service without knowing the transport.
type SubscriptionClient interface {
	FindAllConfirmed(ctx context.Context) ([]domain.Subscription, error)
	UpdateLastSeenTag(ctx context.Context, id int, tag string) error
}
