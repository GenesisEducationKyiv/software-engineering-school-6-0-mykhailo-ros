package main

import (
	"github-release-notifier/internal/db"
	"github-release-notifier/internal/github"
	"github-release-notifier/internal/handler"
	"github-release-notifier/internal/mailer"
	"github-release-notifier/internal/repository"
	"github-release-notifier/internal/service"
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file, using environment variables")
	}
	database, err := db.Connect()
	if err != nil {
		log.Fatalf("failed to connect to db: %v", err)
	}
	defer database.Close()

	if err := db.RunMigrations(database); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}

	log.Println("DB connected and migrations applied")

	repo := repository.NewSubscriptionRepo(database)
	githubClient := github.NewClient(os.Getenv("GITHUB_TOKEN"))
	mailerClient := mailer.NewMailer()

	svc := service.NewSubscription(repo, githubClient, mailerClient)
	h := handler.NewSubscriptionHandler(svc)

	r := gin.Default()
	r.POST("/api/subscribe", h.Subscribe)
	r.GET("/api/confirm/:token", h.Confirm)
	r.GET("/api/unsubscribe/:token", h.Unsubscribe)
	r.GET("/api/subscriptions", h.GetSubscriptions)
	r.Static("/swagger", "./static/swagger")
	r.StaticFile("/swagger.yaml", "./swagger.yaml")

	r.Run(":8080")
}
