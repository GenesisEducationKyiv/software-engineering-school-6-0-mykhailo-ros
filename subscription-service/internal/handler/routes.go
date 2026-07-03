package handler

import (
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"subscription-service/internal/domain"

	"github.com/gin-gonic/gin"
)

var emailRegexp = regexp.MustCompile(`^[^@]+@[^@]+\.[^@]+$`)

type SubscriptionService interface {
	Subscribe(email, repo string) error
	Confirm(token string) error
	Unsubscribe(token string) error
	GetSubscriptions(email string) ([]domain.Subscription, error)
}

type SubscriptionHandler struct {
	service SubscriptionService
}

func NewSubscriptionHandler(service SubscriptionService) *SubscriptionHandler {
	return &SubscriptionHandler{service: service}
}

func (h *SubscriptionHandler) Subscribe(c *gin.Context) {
	var email, repo string

	if c.ContentType() == "application/json" {
		var body struct {
			Email string `json:"email"`
			Repo  string `json:"repo"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
			return
		}
		email = strings.TrimSpace(body.Email)
		repo = strings.TrimSpace(body.Repo)
	} else {
		email = strings.TrimSpace(c.PostForm("email"))
		repo = strings.TrimSpace(c.PostForm("repo"))
	}

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
	if errors.Is(err, domain.ErrRepoNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "repository not found on GitHub"})
		return
	}
	if errors.Is(err, domain.ErrAlreadySubscribed) {
		c.JSON(http.StatusConflict, gin.H{"error": "already subscribed to this repository"})
		return
	}
	if errors.Is(err, domain.ErrRateLimited) {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "github rate limit exceeded, try again later"})
		return
	}

	c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
}

func RequireInternalToken(token string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if token == "" {
			c.Next()
			return
		}
		if c.GetHeader("X-Internal-Token") != token {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		c.Next()
	}
}

func parseToken(c *gin.Context) (string, bool) {
	token := c.Param("token")
	if len(token) != 32 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid token"})
		return "", false
	}
	return token, true
}

func (h *SubscriptionHandler) Confirm(c *gin.Context) {
	token, ok := parseToken(c)
	if !ok {
		return
	}

	err := h.service.Confirm(token)
	if err == nil {
		c.JSON(http.StatusOK, gin.H{"message": "subscription confirmed"})
		return
	}
	if errors.Is(err, domain.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "token not found"})
		return
	}

	c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
}

func (h *SubscriptionHandler) Unsubscribe(c *gin.Context) {
	token, ok := parseToken(c)
	if !ok {
		return
	}

	err := h.service.Unsubscribe(token)
	if err == nil {
		c.JSON(http.StatusOK, gin.H{"message": "unsubscribed successfully"})
		return
	}
	if errors.Is(err, domain.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "invalid token"})
		return
	}

	c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
}

func (h *SubscriptionHandler) GetSubscriptions(c *gin.Context) {
	email := strings.TrimSpace(c.Query("email"))
	if email == "" || !emailRegexp.MatchString(email) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid email"})
		return
	}

	subs, err := h.service.GetSubscriptions(email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
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

// InternalStore is satisfied by *repository.SubscriptionRepo — used for service-to-service endpoints.
type InternalStore interface {
	FindAllConfirmed() ([]domain.Subscription, error)
	UpdateLastSeenTag(id int, tag string) error
}

type InternalHandler struct {
	store InternalStore
}

func NewInternalHandler(store InternalStore) *InternalHandler {
	return &InternalHandler{store: store}
}

func (h *InternalHandler) ListConfirmed(c *gin.Context) {
	subs, err := h.store.FindAllConfirmed()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	type sub struct {
		ID          int    `json:"id"`
		Email       string `json:"email"`
		Repo        string `json:"repo"`
		LastSeenTag string `json:"last_seen_tag"`
	}
	result := make([]sub, len(subs))
	for i, s := range subs {
		result[i] = sub{ID: s.ID, Email: s.Email, Repo: s.Repo, LastSeenTag: s.LastSeenTag}
	}
	c.JSON(http.StatusOK, result)
}

func (h *InternalHandler) UpdateLastSeenTag(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var body struct {
		Tag string `json:"tag"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Tag == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tag is required"})
		return
	}

	if err := h.store.UpdateLastSeenTag(id, body.Tag); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	c.Status(http.StatusNoContent)
}
