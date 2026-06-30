package main

import (
	"context"
	"log/slog"
	"notification-service/internal/cache"
	"notification-service/internal/client"
	"notification-service/internal/config"
	"notification-service/internal/events"
	"notification-service/internal/github"
	"notification-service/internal/mailer"
	"notification-service/internal/scheduler"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/joho/godotenv"
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

	cacheClient := cache.NewCacheWithAddr(cfg.RedisAddr)
	rawGithub := github.NewClient(cfg.GithubToken)
	cachedGithub := github.NewCachingReleaseChecker(rawGithub, cacheClient, 10*time.Minute)
	mailerClient := mailer.NewMailer(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUsername, cfg.SMTPPassword, cfg.SMTPFrom)
	subscriptionClient := client.NewSubscriptionClient(cfg.SubscriptionServiceURL, cfg.InternalToken)

	consumer, err := events.NewConsumer(cfg.RabbitMQURL, mailerClient)
	if err != nil {
		slog.Error("failed to connect to RabbitMQ", "error", err)
		os.Exit(1)
	}
	defer consumer.Close()

	consumer, err := events.NewConsumer(cfg.RabbitMQURL, mailerClient)
	if err != nil {
		slog.Error("failed to connect to RabbitMQ", "error", err)
		os.Exit(1)
	}
	defer consumer.Close()

	notifier := scheduler.NewNotifier(subscriptionClient, cachedGithub, mailerClient)
	sched := scheduler.NewScheduler(notifier, 10*time.Minute)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		consumer.Start(ctx)
	}()
	sched.Start(ctx)
	slog.Info("notification-service started")

	<-ctx.Done()
	slog.Info("notification-service shutting down")
	sched.Wait()
	wg.Wait()
}
