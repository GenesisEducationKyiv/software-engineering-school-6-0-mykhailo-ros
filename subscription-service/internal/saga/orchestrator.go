package saga

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
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
}

func New(repo SubscriptionRepository, publisher Publisher, replies <-chan amqp.Delivery, replyTo string) *Orchestrator {
	return &Orchestrator{
		repo:      repo,
		publisher: publisher,
		replies:   replies,
		replyTo:   replyTo,
		newCorrID: correlationID,
	}
}

func (o *Orchestrator) Execute(email, repo, confirmToken, unsubscribeToken, confirmURL string) error {
	id, err := o.repo.Create(email, repo, confirmToken, unsubscribeToken)
	if err != nil {
		return err
	}

	corrID := o.newCorrID()
	if err := o.publisher.PublishEmailCommand(email, repo, confirmURL, corrID, o.replyTo); err != nil {
		if compErr := o.repo.DeleteByID(id); compErr != nil {
			slog.Error("saga: compensation failed after publish error", "error", compErr)
		}
		return fmt.Errorf("saga: publish command: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			if compErr := o.repo.DeleteByID(id); compErr != nil {
				slog.Error("saga: compensation failed after timeout", "error", compErr)
			}
			return fmt.Errorf("saga: timeout waiting for email confirmation reply")
		case d, ok := <-o.replies:
			if !ok {
				if compErr := o.repo.DeleteByID(id); compErr != nil {
					slog.Error("saga: compensation failed after channel close", "error", compErr)
				}
				return fmt.Errorf("saga: reply channel closed")
			}
			if d.CorrelationId != corrID {
				continue
			}
			if string(d.Body) == "confirmation-email-sent" {
				return o.repo.UpdateStatus(id, "active")
			}
			if compErr := o.repo.DeleteByID(id); compErr != nil {
				slog.Error("saga: compensation failed after email failure", "error", compErr)
			}
			return fmt.Errorf("saga: email confirmation failed")
		}
	}
}
