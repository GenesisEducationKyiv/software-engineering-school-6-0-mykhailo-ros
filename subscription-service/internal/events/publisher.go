package events

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	sagaExchange   = "saga.commands"
	commandRouting = "send-confirmation-email"
)

type Publisher struct {
	conn *amqp.Connection
	ch   *amqp.Channel
	mu   sync.Mutex
}

func NewPublisher(url string) (*Publisher, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("events: dial: %w", err)
	}
	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("events: channel: %w", err)
	}
	if err := ch.ExchangeDeclare(sagaExchange, "direct", true, false, false, false, nil); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("events: declare exchange: %w", err)
	}
	return &Publisher{conn: conn, ch: ch}, nil
}

func (p *Publisher) PublishEmailCommand(email, repo, confirmURL, correlationID, replyTo string) error {
	body, err := json.Marshal(map[string]string{
		"email":       email,
		"repo":        repo,
		"confirm_url": confirmURL,
	})
	if err != nil {
		return fmt.Errorf("events: marshal: %w", err)
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	return p.ch.Publish(sagaExchange, commandRouting, false, false, amqp.Publishing{
		ContentType:   "application/json",
		CorrelationId: correlationID,
		ReplyTo:       replyTo,
		Body:          body,
	})
}

func (p *Publisher) Close() {
	if err := p.ch.Close(); err != nil {
		slog.Warn("events: close channel", "error", err)
	}
	if err := p.conn.Close(); err != nil {
		slog.Warn("events: close connection", "error", err)
	}
}
