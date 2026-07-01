package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"subscription-service/internal/config"
	"subscription-service/internal/db"
	"subscription-service/internal/events"
	"subscription-service/internal/github"
	"subscription-service/internal/handler"
	"subscription-service/internal/metrics"
	"subscription-service/internal/repository"
	"subscription-service/internal/saga"
	"subscription-service/internal/service"
	"syscall"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	if err := godotenv.Load(); err != nil {
		slog.Info("no .env file, using environment variables")
	}

	cfg, err := config.Load()
	if err != nil {
		slog.Error("configuration error", "error", err)
		os.Exit(1)
	}

	database, err := db.Connect(cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName)
	if err != nil {
		slog.Error("failed to connect to db", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := database.Close(); err != nil {
			slog.Error("failed to close db", "error", err)
		}
	}()

	if err := db.RunMigrations(database); err != nil {
		slog.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	slog.Info("DB connected and migrations applied")

	metrics.Init(prometheus.DefaultRegisterer)

	repo := repository.NewSubscriptionRepo(database)
	rawGithub := github.NewClient(cfg.GithubToken)

	publisher, err := events.NewPublisher(cfg.RabbitMQURL)
	if err != nil {
		slog.Error("failed to connect to RabbitMQ", "error", err)
		os.Exit(1)
	}
	defer publisher.Close()

	rc, replyQueueName, err := newReplyConsumer(cfg.RabbitMQURL)
	if err != nil {
		slog.Error("failed to create reply queue", "error", err)
		os.Exit(1)
	}
	defer rc.close()

	replies, err := rc.consume()
	if err != nil {
		slog.Error("failed to start consuming replies", "error", err)
		os.Exit(1)
	}

	orch := saga.New(repo, publisher, replies, replyQueueName)
	svc := service.NewSubscription(repo, rawGithub, orch, cfg.BaseURL)
	h := handler.NewSubscriptionHandler(svc)
	ih := handler.NewInternalHandler(repo)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(metrics.Middleware())
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))
	r.POST("/api/subscribe", h.Subscribe)
	r.GET("/api/confirm/:token", h.Confirm)
	r.GET("/api/unsubscribe/:token", h.Unsubscribe)
	r.GET("/api/subscriptions", h.GetSubscriptions)

	internal := r.Group("/internal")
	internal.Use(handler.RequireInternalToken(cfg.InternalToken))
	internal.GET("/subscriptions", ih.ListConfirmed)
	internal.PATCH("/subscriptions/:id/last-seen-tag", ih.UpdateLastSeenTag)

	r.StaticFile("/", "./static/index.html")
	r.Static("/swagger", "./static/swagger")
	r.StaticFile("/swagger.yaml", "./swagger.yaml")

	srv := &http.Server{
		Addr:              ":8080",
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server error", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down server")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("server shutdown", "error", err)
	}
}

type replyConsumerHandle struct {
	conn *amqp.Connection
	ch   *amqp.Channel
	name string
}

func newReplyConsumer(url string) (*replyConsumerHandle, string, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, "", fmt.Errorf("reply queue: dial: %w", err)
	}
	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, "", fmt.Errorf("reply queue: channel: %w", err)
	}
	q, err := ch.QueueDeclare("", false, true, true, false, nil)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, "", fmt.Errorf("reply queue: declare: %w", err)
	}
	return &replyConsumerHandle{conn: conn, ch: ch, name: q.Name}, q.Name, nil
}

func (rc *replyConsumerHandle) consume() (<-chan amqp.Delivery, error) {
	return rc.ch.Consume(rc.name, "", true, true, false, false, nil)
}

func (rc *replyConsumerHandle) close() {
	rc.ch.Close()
	rc.conn.Close()
}
