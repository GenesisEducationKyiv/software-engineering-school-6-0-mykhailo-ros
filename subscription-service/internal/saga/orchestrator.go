package saga

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	replyEmailSent   = "confirmation-email-sent"
	replyEmailFailed = "confirmation-email-failed"
)

type SubscriptionRepository interface {
	Create(email, repo, confirmToken, unsubscribeToken string) (int, error)
	UpdateStatus(id int, status string) error
	DeleteByID(id int) error
}

type Publisher interface {
	PublishEmailCommand(email, repo, confirmURL, correlationID, replyTo string) error
}

type Orchestrator struct {
	repo      SubscriptionRepository
	publisher Publisher
	replies   <-chan amqp.Delivery
	replyTo   string
	newCorrID func() string

	mu      sync.Mutex
	pending map[string]chan amqp.Delivery
}

func New(repo SubscriptionRepository, publisher Publisher, replies <-chan amqp.Delivery, replyTo string) *Orchestrator {
	return &Orchestrator{
		repo:      repo,
		publisher: publisher,
		replies:   replies,
		replyTo:   replyTo,
		newCorrID: correlationID,
		pending:   make(map[string]chan amqp.Delivery),
	}
}

// Start runs the reply dispatcher, routing each delivery to the Execute call
// waiting on its correlation ID. It must be running before any Execute call
// can receive a reply.
func (o *Orchestrator) Start(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case d, ok := <-o.replies:
			if !ok {
				return
			}
			o.mu.Lock()
			ch, found := o.pending[d.CorrelationId]
			o.mu.Unlock()
			if found {
				ch <- d
			}
		}
	}
}

func (o *Orchestrator) Execute(ctx context.Context, email, repo, confirmToken, unsubscribeToken, confirmURL string) error {
	id, err := o.repo.Create(email, repo, confirmToken, unsubscribeToken)
	if err != nil {
		return err
	}

	corrID := o.newCorrID()
	replyCh := make(chan amqp.Delivery, 1)

	o.mu.Lock()
	o.pending[corrID] = replyCh
	o.mu.Unlock()
	defer func() {
		o.mu.Lock()
		delete(o.pending, corrID)
		o.mu.Unlock()
	}()

	if err := o.publisher.PublishEmailCommand(email, repo, confirmURL, corrID, o.replyTo); err != nil {
		if compErr := o.repo.DeleteByID(id); compErr != nil {
			slog.Error("saga: compensation failed after publish error", "error", compErr)
		}
		return fmt.Errorf("saga: publish command: %w", err)
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()

	select {
	case <-timeoutCtx.Done():
		if compErr := o.repo.DeleteByID(id); compErr != nil {
			slog.Error("saga: compensation failed after timeout", "error", compErr)
		}
		return fmt.Errorf("saga: timeout waiting for email confirmation reply")
	case d := <-replyCh:
		if string(d.Body) == replyEmailSent {
			return o.repo.UpdateStatus(id, "active")
		}
		if compErr := o.repo.DeleteByID(id); compErr != nil {
			slog.Error("saga: compensation failed after email failure", "error", compErr)
		}
		return fmt.Errorf("saga: email confirmation failed")
	}
}
