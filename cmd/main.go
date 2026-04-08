package main

import (
	"github-release-notifier/internal/db"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

func main() {
	database, err := db.Connect()
	if err != nil {
		log.Fatalf("failed to connect to db: %v", err)
	}
	defer database.Close()

	if err := db.RunMigrations(database); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}

	log.Println("DB connected and migrations applied")

	r := gin.Default()

	r.POST("/api/subscribe", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	r.GET("/api/confirm/:token", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	r.GET("/api/unsubscribe/:token", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	r.GET("/api/subscriptions", func(c *gin.Context) {
		c.JSON(http.StatusOK, []gin.H{})
	})

	r.Run(":8080")
}
