package service

import (
	"fmt"
	"testing"

	"subscription-service/internal/domain"
)

type mockGithub struct {
	exists bool
	err    error
}

func (m *mockGithub) RepoExists(repo string) (bool, error) {
	return m.exists, m.err
}

type mockPublisher struct {
	called bool
	err    error
}

func (m *mockPublisher) PublishSubscriptionCreated(email, repo, confirmURL string) error {
	m.called = true
	return m.err
}

type mockRepo struct {
	subscription  *domain.Subscription
	subscriptions []domain.Subscription
	createErr     error
	confirmErr    error
	deleteErr     error
}

func (m *mockRepo) Create(email, repo, confirmToken, unsubscribeToken string) error {
	return m.createErr
}

func (m *mockRepo) FindByConfirmToken(token string) (*domain.Subscription, error) {
	if m.subscription == nil {
		return nil, domain.ErrNotFound
	}
	return m.subscription, nil
}

func (m *mockRepo) FindByUnsubscribeToken(token string) (*domain.Subscription, error) {
	if m.subscription == nil {
		return nil, domain.ErrNotFound
	}
	return m.subscription, nil
}

func (m *mockRepo) Confirm(token string) error {
	return m.confirmErr
}

func (m *mockRepo) DeleteByUnsubscribeToken(token string) error {
	return m.deleteErr
}

func (m *mockRepo) FindByEmail(email string) ([]domain.Subscription, error) {
	return m.subscriptions, nil
}


func TestSubscribe_Success(t *testing.T) {
	svc := NewSubscription(
		&mockRepo{},
		&mockGithub{exists: true},
		&mockPublisher{},
		"",
	)

	if err := svc.Subscribe("test@test.com", "golang/go"); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestSubscribe_RepoNotFound(t *testing.T) {
	svc := NewSubscription(
		&mockRepo{},
		&mockGithub{exists: false},
		&mockPublisher{},
		"",
	)

	err := svc.Subscribe("test@test.com", "golang/go")
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestSubscribe_GithubError(t *testing.T) {
	svc := NewSubscription(
		&mockRepo{},
		&mockGithub{err: fmt.Errorf("rate limit exceeded")},
		&mockPublisher{},
		"",
	)

	err := svc.Subscribe("test@test.com", "golang/go")
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestConfirm_Success(t *testing.T) {
	svc := NewSubscription(
		&mockRepo{subscription: &domain.Subscription{ID: 1}},
		&mockGithub{},
		&mockPublisher{},
		"",
	)

	if err := svc.Confirm("valid-token"); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestConfirm_TokenNotFound(t *testing.T) {
	svc := NewSubscription(
		&mockRepo{subscription: nil},
		&mockGithub{},
		&mockPublisher{},
		"",
	)

	err := svc.Confirm("invalid-token")
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestUnsubscribe_Success(t *testing.T) {
	svc := NewSubscription(
		&mockRepo{subscription: &domain.Subscription{ID: 1}},
		&mockGithub{},
		&mockPublisher{},
		"",
	)

	if err := svc.Unsubscribe("valid-token"); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestUnsubscribe_TokenNotFound(t *testing.T) {
	svc := NewSubscription(
		&mockRepo{subscription: nil},
		&mockGithub{},
		&mockPublisher{},
		"",
	)

	err := svc.Unsubscribe("invalid-token")
	if err == nil {
		t.Error("expected error, got nil")
	}
}
