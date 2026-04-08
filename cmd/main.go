package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func main() {
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
