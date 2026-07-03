package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	sagaExchange   = "saga.commands"
	commandRouting = "send-confirmation-email"

	replyEmailSent   = "confirmation-email-sent"
	replyEmailFailed = "confirmation-email-failed"
)

type Mailer interface {
	SendConfirmation(to, repo, confirmURL string) error
}

type Consumer struct {
	conn   *amqp.Connection
	ch     *amqp.Channel
	mailer Mailer
	msgs   <-chan amqp.Delivery
}

func NewConsumer(url string, m Mailer) (*Consumer, error) {
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
	q, err := ch.QueueDeclare("", false, true, true, false, nil)
	if err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("events: declare queue: %w", err)
	}
	if err := ch.QueueBind(q.Name, commandRouting, sagaExchange, false, nil); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("events: bind queue: %w", err)
	}
	msgs, err := ch.Consume(q.Name, "", false, true, false, false, nil)
	if err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("events: consume: %w", err)
	}
	return &Consumer{conn: conn, ch: ch, mailer: m, msgs: msgs}, nil
}

func (c *Consumer) Start(ctx context.Context) {
	slog.Info("events: consumer started")
	for {
		select {
		case <-ctx.Done():
			return
		case d, ok := <-c.msgs:
			if !ok {
				return
			}
			reply := replyEmailSent
			if err := c.handle(d.Body); err != nil {
				slog.Error("events: handle command", "error", err)
				reply = replyEmailFailed
			}
			if err := c.ch.Publish("", d.ReplyTo, false, false, amqp.Publishing{
				CorrelationId: d.CorrelationId,
				Body:          []byte(reply),
			}); err != nil {
				slog.Error("events: publish reply", "error", err)
				if nackErr := d.Nack(false, false); nackErr != nil {
					slog.Error("events: nack message", "error", nackErr)
				}
				continue
			}
			if err := d.Ack(false); err != nil {
				slog.Error("events: ack message", "error", err)
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
	if err := c.ch.Close(); err != nil {
		slog.Warn("events: close channel", "error", err)
	}
	if err := c.conn.Close(); err != nil {
		slog.Warn("events: close connection", "error", err)
	}
}
