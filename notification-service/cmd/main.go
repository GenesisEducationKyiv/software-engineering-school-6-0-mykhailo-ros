package main

import (
	"context"
	"log/slog"
	"notification-service/internal/cache"
	"notification-service/internal/client"
	"notification-service/internal/config"
	"notification-service/internal/github"
	"notification-service/internal/mailer"
	"notification-service/internal/scheduler"
	"os"
	"os/signal"
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
	subscriptionClient := client.NewSubscriptionClient(cfg.SubscriptionServiceURL)

	notifier := scheduler.NewNotifier(subscriptionClient, cachedGithub, mailerClient)
	sched := scheduler.NewScheduler(notifier, 10*time.Minute)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	sched.Start(ctx)
	slog.Info("notification-service started")

	<-ctx.Done()
	slog.Info("notification-service shutting down")
}
