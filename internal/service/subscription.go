package service

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"github-release-notifier/internal/github"
	"github-release-notifier/internal/mailer"
	"github-release-notifier/internal/repository"
	"os"
)

type Subscription struct {
	repo   *repository.SubscriptionRepo
	github *github.Client
	mailer *mailer.Mailer
}

func NewSubscription(repo *repository.SubscriptionRepo, github *github.Client, mailer *mailer.Mailer) *Subscription {
	return &Subscription{repo: repo, github: github, mailer: mailer}
}

func generateToken() (string, error) {
	bytes := make([]byte, 16)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func (s *Subscription) Subscribe(email, repo string) error {
	exists, err := s.github.RepoExists(repo)
	if err != nil {
		return fmt.Errorf("github: %w", err)
	}
	if !exists {
		return fmt.Errorf("repo not found")
	}

	confirmToken, err := generateToken()
	if err != nil {
		return err
	}
	unsubscribeToken, err := generateToken()
	if err != nil {
		return err
	}
	err = s.repo.Create(email, repo, confirmToken, unsubscribeToken)
	if err != nil {
		return err
	}

	baseURL := os.Getenv("BASE_URL")
	confirmURL := fmt.Sprintf("%s/api/confirm/%s", baseURL, confirmToken)
	return s.mailer.SendConfirmation(email, repo, confirmURL)
}

func (s *Subscription) Confirm(token string) error {
	sub, err := s.repo.FindByConfirmToken(token)
	if err != nil {
		return err
	}
	if sub == nil {
		return fmt.Errorf("token not found")
	}
	return s.repo.Confirm(token)
}

func (s *Subscription) Unsubscribe(token string) error {
	sub, err := s.repo.FindByUnsubscribeToken(token)
	if err != nil {
		return err
	}
	if sub == nil {
		return fmt.Errorf("token not found")
	}
	return s.repo.DeleteByUnsubscribeToken(token)
}

func (s *Subscription) GetSubscriptions(email string) ([]repository.Subscription, error) {
	return s.repo.FindByEmail(email)
}
