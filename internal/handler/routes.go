package handler

import (
	"github-release-notifier/internal/service"
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
)

var emailRegexp = regexp.MustCompile(`^[^@]+@[^@]+\.[^@]+$`)

type SubscriptionHandler struct {
	service *service.Subscription
}

func NewSubscriptionHandler(service *service.Subscription) *SubscriptionHandler {
	return &SubscriptionHandler{service: service}
}

func (h *SubscriptionHandler) Subscribe(c *gin.Context) {
	email := strings.TrimSpace(c.PostForm("email"))
	repo := strings.TrimSpace(c.PostForm("repo"))

	if email == "" || repo == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email and repo are required"})
		return
	}
	if !emailRegexp.MatchString(email) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "wrong email format"})
		return
	}
	if !strings.Contains(repo, "/") || len(strings.Split(repo, "/")) != 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid repo format, use owner/repo"})
		return
	}

	err := h.service.Subscribe(email, repo)
	if err == nil {
		c.JSON(http.StatusOK, gin.H{"message": "subscription created, check your email"})
		return
	}
	if strings.Contains(err.Error(), "repo not found") {
		c.JSON(http.StatusNotFound, gin.H{"error": "repository not found on GitHub"})
		return
	}
	if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
		c.JSON(http.StatusConflict, gin.H{"error": "already subscribed to this repository"})
		return
	}
	if strings.Contains(err.Error(), "rate limit") {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "github rate limit exceeded, try again later"})
		return
	}

	c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
}

func (h *SubscriptionHandler) Confirm(c *gin.Context) {
	token := c.Param("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid token"})
		return
	}

	err := h.service.Confirm(token)
	if err == nil {
		c.JSON(http.StatusOK, gin.H{"message": "subscription confirmed"})
		return
	}
	if strings.Contains(err.Error(), "token not found") {
		c.JSON(http.StatusNotFound, gin.H{"error": "invalid token"})
		return
	}

	c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
}

func (h *SubscriptionHandler) Unsubscribe(c *gin.Context) {
	token := c.Param("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid token"})
		return
	}

	err := h.service.Unsubscribe(token)
	if err == nil {
		c.JSON(http.StatusOK, gin.H{"message": "unsubscribed successfully"})
		return
	}
	if strings.Contains(err.Error(), "token not found") {
		c.JSON(http.StatusNotFound, gin.H{"error": "invalid token"})
		return
	}

	c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
}

func (h *SubscriptionHandler) GetSubscriptions(c *gin.Context) {
	email := strings.TrimSpace(c.Query("email"))
	if email == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email is required"})
	}

	subs, err := h.service.GetSubscriptions(email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	}

	type response struct {
		Email       string `json:"email"`
		Repo        string `json:"repo"`
		Confirmed   bool   `json:"confirmed"`
		LastSeenTag string `json:"last_seen_tag"`
	}

	result := make([]response, len(subs))
	for i, sub := range subs {
		result[i] = response{
			Email:       sub.Email,
			Repo:        sub.Repo,
			Confirmed:   sub.Confirmed,
			LastSeenTag: sub.LastSeenTag,
		}
	}

	c.JSON(http.StatusOK, result)
}
