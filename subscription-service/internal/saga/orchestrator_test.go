package saga

import (
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
	err error
}

func (m *mockPublisher) PublishEmailCommand(email, repo, confirmURL, correlationID, replyTo string) error {
	return m.err
}

const fixedCorrID = "test-corr-id-fixed"

func newTestOrchestrator(repo SubscriptionRepository, pub Publisher, replies <-chan amqp.Delivery) *Orchestrator {
	o := New(repo, pub, replies, "reply-queue")
	o.newCorrID = func() string { return fixedCorrID }
	return o
}

func TestExecute_Success(t *testing.T) {
	repo := &mockRepo{createID: 1}
	pub := &mockPublisher{}
	replyCh := make(chan amqp.Delivery, 1)
	replyCh <- amqp.Delivery{CorrelationId: fixedCorrID, Body: []byte("confirmation-email-sent")}

	orch := newTestOrchestrator(repo, pub, replyCh)
	if err := orch.Execute("a@b.com", "owner/repo", "tok", "unsub", "http://x/confirm/tok"); err != nil {
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
	pub := &mockPublisher{}
	replyCh := make(chan amqp.Delivery, 1)
	replyCh <- amqp.Delivery{CorrelationId: fixedCorrID, Body: []byte("confirmation-email-failed")}

	orch := newTestOrchestrator(repo, pub, replyCh)
	err := orch.Execute("a@b.com", "owner/repo", "tok", "unsub", "http://x/confirm/tok")

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
	replyCh := make(chan amqp.Delivery, 1)

	orch := newTestOrchestrator(repo, pub, replyCh)
	err := orch.Execute("a@b.com", "owner/repo", "tok", "unsub", "http://x/confirm/tok")

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
	replyCh := make(chan amqp.Delivery, 1)

	orch := newTestOrchestrator(repo, pub, replyCh)
	err := orch.Execute("a@b.com", "owner/repo", "tok", "unsub", "http://x/confirm/tok")

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if repo.deleteCalls != 0 {
		t.Errorf("expected no compensation on create error, got %d DeleteByID calls", repo.deleteCalls)
	}
}

func TestExecute_UnrelatedReplyIgnored(t *testing.T) {
	repo := &mockRepo{createID: 1}
	pub := &mockPublisher{}
	replyCh := make(chan amqp.Delivery, 2)
	replyCh <- amqp.Delivery{CorrelationId: "other-id", Body: []byte("confirmation-email-sent")}
	replyCh <- amqp.Delivery{CorrelationId: fixedCorrID, Body: []byte("confirmation-email-sent")}

	orch := newTestOrchestrator(repo, pub, replyCh)
	if err := orch.Execute("a@b.com", "owner/repo", "tok", "unsub", "http://x/confirm/tok"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if repo.updateCalls != 1 {
		t.Errorf("expected UpdateStatus called once, got %d", repo.updateCalls)
	}
}
