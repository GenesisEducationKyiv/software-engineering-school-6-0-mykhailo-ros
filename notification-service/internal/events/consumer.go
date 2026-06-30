package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	amqp "github.com/rabbitmq/amqp091-go"
)

const exchangeName = "subscription.events"

type Mailer interface {
	SendConfirmation(to, repo, confirmURL string) error
}

type Consumer struct {
	conn   *amqp.Connection
	ch     *amqp.Channel
	mailer Mailer
}

func NewConsumer(url string, m Mailer) (*Consumer, error) {
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
	return &Consumer{conn: conn, ch: ch, mailer: m}, nil
}

func (c *Consumer) Start(ctx context.Context) {
	q, err := c.ch.QueueDeclare("", false, true, true, false, nil)
	if err != nil {
		slog.Error("events: declare queue", "error", err)
		return
	}
	if err := c.ch.QueueBind(q.Name, "", exchangeName, false, nil); err != nil {
		slog.Error("events: bind queue", "error", err)
		return
	}
	msgs, err := c.ch.Consume(q.Name, "", true, true, false, false, nil)
	if err != nil {
		slog.Error("events: consume", "error", err)
		return
	}
	slog.Info("events: consumer started")
	for {
		select {
		case <-ctx.Done():
			return
		case d, ok := <-msgs:
			if !ok {
				return
			}
			if err := c.handle(d.Body); err != nil {
				slog.Error("events: handle message", "error", err)
			}
		}
	}
}

func (c *Consumer) handle(body []byte) error {
	var msg struct {
		Email      string `json:"email"`
		Repo       string `json:"repo"`
		ConfirmURL string `json:"confirm_url"`
	}
	if err := json.Unmarshal(body, &msg); err != nil {
		return fmt.Errorf("events: unmarshal: %w", err)
	}
	return c.mailer.SendConfirmation(msg.Email, msg.Repo, msg.ConfirmURL)
}

func (c *Consumer) Close() {
	c.ch.Close()
	c.conn.Close()
}
