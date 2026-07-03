package saga

import (
	"context"
	"errors"
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
)

var (
	errPublish = errors.New("publish failed")
	errDB      = errors.New("db error")
)

type mockRepo struct {
	createID    int
	createErr   error
	updateErr   error
	deleteErr   error
	deleteCalls int
	updateCalls int
}

func (m *mockRepo) Create(email, repo, confirmToken, unsubscribeToken string) (int, error) {
	return m.createID, m.createErr
}

func (m *mockRepo) UpdateStatus(id int, status string) error {
	m.updateCalls++
	return m.updateErr
}

func (m *mockRepo) DeleteByID(id int) error {
	m.deleteCalls++
	return m.deleteErr
}

type mockPublisher struct {
	err       error
	onPublish func(corrID string)
}

func (m *mockPublisher) PublishEmailCommand(email, repo, confirmURL, correlationID, replyTo string) error {
	if m.err != nil {
		return m.err
	}
	if m.onPublish != nil {
		m.onPublish(correlationID)
	}
	return nil
}

const fixedCorrID = "test-corr-id-fixed"

func newTestOrchestrator(repo SubscriptionRepository, pub Publisher, replies <-chan amqp.Delivery) *Orchestrator {
	o := New(repo, pub, replies, "reply-queue")
	o.newCorrID = func() string { return fixedCorrID }
	return o
}

func TestExecute_Success(t *testing.T) {
	repo := &mockRepo{createID: 1}
	replies := make(chan amqp.Delivery, 1)
	pub := &mockPublisher{onPublish: func(corrID string) {
		replies <- amqp.Delivery{CorrelationId: corrID, Body: []byte(replyEmailSent)}
	}}

	orch := newTestOrchestrator(repo, pub, replies)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go orch.Start(ctx)

	if err := orch.Execute(context.Background(), "a@b.com", "owner/repo", "tok", "unsub", "http://x/confirm/tok"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if repo.updateCalls != 1 {
		t.Errorf("expected UpdateStatus called once, got %d", repo.updateCalls)
	}
	if repo.deleteCalls != 0 {
		t.Errorf("expected no compensation, got %d DeleteByID calls", repo.deleteCalls)
	}
}

func TestExecute_EmailFailed_Compensation(t *testing.T) {
	repo := &mockRepo{createID: 7}
	replies := make(chan amqp.Delivery, 1)
	pub := &mockPublisher{onPublish: func(corrID string) {
		replies <- amqp.Delivery{CorrelationId: corrID, Body: []byte(replyEmailFailed)}
	}}

	orch := newTestOrchestrator(repo, pub, replies)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go orch.Start(ctx)

	err := orch.Execute(context.Background(), "a@b.com", "owner/repo", "tok", "unsub", "http://x/confirm/tok")

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if repo.deleteCalls != 1 {
		t.Errorf("expected 1 compensation call, got %d", repo.deleteCalls)
	}
}

func TestExecute_PublishError_Compensation(t *testing.T) {
	repo := &mockRepo{createID: 42}
	pub := &mockPublisher{err: errPublish}
	replies := make(chan amqp.Delivery, 1)

	orch := newTestOrchestrator(repo, pub, replies)
	err := orch.Execute(context.Background(), "a@b.com", "owner/repo", "tok", "unsub", "http://x/confirm/tok")

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if repo.deleteCalls != 1 {
		t.Errorf("expected 1 compensation call, got %d", repo.deleteCalls)
	}
}

func TestExecute_CreateError_NoCompensation(t *testing.T) {
	repo := &mockRepo{createErr: errDB}
	pub := &mockPublisher{}
	replies := make(chan amqp.Delivery, 1)

	orch := newTestOrchestrator(repo, pub, replies)
	err := orch.Execute(context.Background(), "a@b.com", "owner/repo", "tok", "unsub", "http://x/confirm/tok")

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if repo.deleteCalls != 0 {
		t.Errorf("expected no compensation on create error, got %d DeleteByID calls", repo.deleteCalls)
	}
}

func TestExecute_UnrelatedReplyIgnored(t *testing.T) {
	repo := &mockRepo{createID: 1}
	replies := make(chan amqp.Delivery, 2)
	pub := &mockPublisher{onPublish: func(corrID string) {
		replies <- amqp.Delivery{CorrelationId: "other-id", Body: []byte(replyEmailSent)}
		replies <- amqp.Delivery{CorrelationId: corrID, Body: []byte(replyEmailSent)}
	}}

	orch := newTestOrchestrator(repo, pub, replies)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go orch.Start(ctx)

	if err := orch.Execute(context.Background(), "a@b.com", "owner/repo", "tok", "unsub", "http://x/confirm/tok"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if repo.updateCalls != 1 {
		t.Errorf("expected UpdateStatus called once, got %d", repo.updateCalls)
	}
}

func TestExecute_Timeout(t *testing.T) {
	repo := &mockRepo{createID: 5}
	pub := &mockPublisher{}
	replies := make(chan amqp.Delivery, 1)

	orch := newTestOrchestrator(repo, pub, replies)

	execCtx, execCancel := context.WithCancel(context.Background())
	execCancel()

	err := orch.Execute(execCtx, "a@b.com", "owner/repo", "tok", "unsub", "http://x/confirm/tok")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if repo.deleteCalls != 1 {
		t.Errorf("expected 1 compensation call, got %d", repo.deleteCalls)
	}
	if repo.updateCalls != 0 {
		t.Errorf("expected no UpdateStatus call, got %d", repo.updateCalls)
	}
}

func TestExecute_ChannelClosed(t *testing.T) {
	repo := &mockRepo{createID: 9}
	pub := &mockPublisher{}
	replies := make(chan amqp.Delivery, 1)
	close(replies)

	orch := newTestOrchestrator(repo, pub, replies)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go orch.Start(ctx)

	execCtx, execCancel := context.WithCancel(context.Background())
	execCancel()

	err := orch.Execute(execCtx, "a@b.com", "owner/repo", "tok", "unsub", "http://x/confirm/tok")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if repo.deleteCalls != 1 {
		t.Errorf("expected 1 compensation call, got %d", repo.deleteCalls)
	}
}
