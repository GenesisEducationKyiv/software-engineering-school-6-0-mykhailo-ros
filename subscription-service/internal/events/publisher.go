package events

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"
)

const exchangeName = "subscription.events"

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
		conn.Close()
		return nil, fmt.Errorf("events: channel: %w", err)
	}
	if err := ch.ExchangeDeclare(exchangeName, "fanout", true, false, false, false, nil); err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("events: declare exchange: %w", err)
	}
	return &Publisher{conn: conn, ch: ch}, nil
}

func (p *Publisher) PublishSubscriptionCreated(email, repo, confirmURL string) error {
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
	return p.ch.Publish(exchangeName, "", false, false, amqp.Publishing{
		ContentType: "application/json",
		Body:        body,
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
