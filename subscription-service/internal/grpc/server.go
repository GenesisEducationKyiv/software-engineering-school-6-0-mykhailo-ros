package grpc

import (
	"context"
	"log/slog"

	subscriptionv1 "github-release-notifier/gen/subscription/v1"

	"subscription-service/internal/domain"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type SubscriptionRepository interface {
	FindAllConfirmed() ([]domain.Subscription, error)
	UpdateLastSeenTag(id int, tag string) error
}

type Server struct {
	subscriptionv1.UnimplementedSubscriptionServiceServer
	repo SubscriptionRepository
}

func NewServer(repo SubscriptionRepository) *Server {
	return &Server{repo: repo}
}

func (s *Server) ListConfirmedSubscriptions(ctx context.Context, _ *subscriptionv1.ListConfirmedSubscriptionsRequest) (*subscriptionv1.ListConfirmedSubscriptionsResponse, error) {
	subs, err := s.repo.FindAllConfirmed()
	if err != nil {
		slog.Error("grpc: list confirmed subscriptions", "error", err)
		return nil, status.Error(codes.Internal, "internal server error")
	}

	resp := &subscriptionv1.ListConfirmedSubscriptionsResponse{
		Subscriptions: make([]*subscriptionv1.Subscription, len(subs)),
	}
	for i, sub := range subs {
		resp.Subscriptions[i] = &subscriptionv1.Subscription{
			Id:          int64(sub.ID),
			Email:       sub.Email,
			Repo:        sub.Repo,
			LastSeenTag: sub.LastSeenTag,
		}
	}
	return resp, nil
}

func (s *Server) UpdateLastSeenTag(ctx context.Context, req *subscriptionv1.UpdateLastSeenTagRequest) (*subscriptionv1.UpdateLastSeenTagResponse, error) {
	if err := s.repo.UpdateLastSeenTag(int(req.GetId()), req.GetLastSeenTag()); err != nil {
		slog.Error("grpc: update last seen tag", "error", err)
		return nil, status.Error(codes.Internal, "internal server error")
	}
	return &subscriptionv1.UpdateLastSeenTagResponse{}, nil
}
