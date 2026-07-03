package main

import (
	"context"
	"github-release-notifier/internal/cache"
	"github-release-notifier/internal/db"
	"github-release-notifier/internal/github"
	"github-release-notifier/internal/handler"
	"github-release-notifier/internal/mailer"
	"github-release-notifier/internal/repository"
	"github-release-notifier/internal/scheduler"
	"github-release-notifier/internal/service"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file, using environment variables")
	}
	database, err := db.Connect(
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
	)
	if err != nil {
		log.Fatalf("failed to connect to db: %v", err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			log.Printf("failed to close db: %v", err)
		}
	}()

	if err := db.RunMigrations(database); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}

	log.Println("DB connected and migrations applied")

	repo := repository.NewSubscriptionRepo(database)
	cacheClient := cache.NewCache()
	githubClient := github.NewClient(os.Getenv("GITHUB_TOKEN"), cacheClient)
	mailerClient := mailer.NewMailer(
		os.Getenv("SMTP_HOST"),
		os.Getenv("SMTP_PORT"),
		os.Getenv("SMTP_USERNAME"),
		os.Getenv("SMTP_PASSWORD"),
		os.Getenv("SMTP_FROM"),
	)

	svc := service.NewSubscription(repo, githubClient, mailerClient, os.Getenv("BASE_URL"))
	h := handler.NewSubscriptionHandler(svc)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	notifier := scheduler.NewNotifier(repo, githubClient, mailerClient)
	sched := scheduler.NewScheduler(notifier, 10*time.Minute)
	sched.Start(ctx)

	r := gin.Default()
	r.POST("/api/subscribe", h.Subscribe)
	r.GET("/api/confirm/:token", h.Confirm)
	r.GET("/api/unsubscribe/:token", h.Unsubscribe)
	r.GET("/api/subscriptions", h.GetSubscriptions)
	r.Static("/swagger", "./static/swagger")
	r.StaticFile("/swagger.yaml", "./swagger.yaml")

	if err := r.Run(":8080"); err != nil {
		log.Fatal(err)
	}
}
