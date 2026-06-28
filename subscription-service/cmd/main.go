package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"subscription-service/internal/config"
	"subscription-service/internal/db"
	"subscription-service/internal/github"
	"subscription-service/internal/handler"
	"subscription-service/internal/mailer"
	"subscription-service/internal/metrics"
	"subscription-service/internal/repository"
	"subscription-service/internal/service"
	"syscall"
	"time"

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
	mailerClient := mailer.NewMailer(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUsername, cfg.SMTPPassword, cfg.SMTPFrom)

	svc := service.NewSubscription(repo, rawGithub, mailerClient, cfg.BaseURL)
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
	r.GET("/internal/subscriptions", ih.ListConfirmed)
	r.PATCH("/internal/subscriptions/:id/last-seen-tag", ih.UpdateLastSeenTag)
	r.StaticFile("/", "./static/index.html")
	r.Static("/swagger", "./static/swagger")
	r.StaticFile("/swagger.yaml", "./swagger.yaml")

	srv := &http.Server{Addr: ":8080", Handler: r}
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server error", "error", err)
			os.Exit(1)
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
