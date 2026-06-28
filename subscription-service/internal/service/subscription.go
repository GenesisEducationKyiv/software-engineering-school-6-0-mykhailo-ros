package service

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"subscription-service/internal/domain"
)

type GithubClient interface {
	RepoExists(repo string) (bool, error)
}

type Mailer interface {
	SendConfirmation(to, repo, confirmURL string) error
}

type SubscriptionRepository interface {
	Create(email, repo, confirmToken, unsubscribeToken string) error
	FindByConfirmToken(token string) (*domain.Subscription, error)
	FindByUnsubscribeToken(token string) (*domain.Subscription, error)
	Confirm(token string) error
	DeleteByUnsubscribeToken(token string) error
	FindByEmail(email string) ([]domain.Subscription, error)
}

type Subscription struct {
	repo    SubscriptionRepository
	github  GithubClient
	mailer  Mailer
	baseURL string
}

func NewSubscription(repo SubscriptionRepository, github GithubClient, mailer Mailer, baseURL string) *Subscription {
	return &Subscription{repo: repo, github: github, mailer: mailer, baseURL: baseURL}
}

func (s *Subscription) Subscribe(email, repo string) error {
	exists, err := s.github.RepoExists(repo)
	if err != nil {
		return fmt.Errorf("github: %w", err)
	}
	if !exists {
		return domain.ErrRepoNotFound
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

	confirmURL := fmt.Sprintf("%s/api/confirm/%s", s.baseURL, confirmToken)
	return s.mailer.SendConfirmation(email, repo, confirmURL)
}

func (s *Subscription) Confirm(token string) error {
	if _, err := s.repo.FindByConfirmToken(token); err != nil {
		return err
	}
	return s.repo.Confirm(token)
}

func (s *Subscription) Unsubscribe(token string) error {
	if _, err := s.repo.FindByUnsubscribeToken(token); err != nil {
		return err
	}
	return s.repo.DeleteByUnsubscribeToken(token)
}

func (s *Subscription) GetSubscriptions(email string) ([]domain.Subscription, error) {
	return s.repo.FindByEmail(email)
}

func generateToken() (string, error) {
	bytes := make([]byte, 16)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
