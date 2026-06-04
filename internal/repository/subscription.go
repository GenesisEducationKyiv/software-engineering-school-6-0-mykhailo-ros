package repository

import (
	"database/sql"
	"errors"
	"log"
)

var ErrNotFound = errors.New("not found")

type Subscription struct {
	ID               int
	Email            string
	Repo             string
	Confirmed        bool
	ConfirmToken     string
	UnsubscribeToken string
	LastSeenTag      string
}

type SubscriptionRepo struct {
	db *sql.DB
}

func NewSubscriptionRepo(db *sql.DB) *SubscriptionRepo {
	return &SubscriptionRepo{db: db}
}

func (r *SubscriptionRepo) Create(email, repo, confirmToken, unsubscribeToken string) error {
	_, err := r.db.Exec(`
		INSERT INTO subscriptions (email, repo, confirm_token, unsubscribe_token)
		VALUES ($1, $2, $3, $4)`,
		email, repo, confirmToken, unsubscribeToken,
	)
	return err
}

func (r *SubscriptionRepo) FindByConfirmToken(token string) (*Subscription, error) {
	s := &Subscription{}
	err := r.db.QueryRow(`
		SELECT id, email, repo, confirmed, confirm_token, unsubscribe_token, last_seen_tag
		FROM subscriptions WHERE confirm_token = $1`, token).
		Scan(&s.ID, &s.Email, &s.Repo, &s.Confirmed, &s.ConfirmToken, &s.UnsubscribeToken, &s.LastSeenTag)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return s, err
}

func (r *SubscriptionRepo) FindByUnsubscribeToken(token string) (*Subscription, error) {
	s := &Subscription{}
	err := r.db.QueryRow(`
		SELECT id, email, repo, confirmed, confirm_token, unsubscribe_token, last_seen_tag
		FROM subscriptions WHERE unsubscribe_token = $1`, token).
		Scan(&s.ID, &s.Email, &s.Repo, &s.Confirmed, &s.ConfirmToken, &s.UnsubscribeToken, &s.LastSeenTag)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return s, err
}

func (r *SubscriptionRepo) Confirm(token string) error {
	_, err := r.db.Exec(
		`UPDATE subscriptions SET confirmed = true WHERE confirm_token = $1`, token)
	return err
}

func (r *SubscriptionRepo) DeleteByUnsubscribeToken(token string) error {
	_, err := r.db.Exec(
		`DELETE FROM subscriptions WHERE unsubscribe_token = $1`, token)
	return err
}

func (r *SubscriptionRepo) FindByEmail(email string) ([]Subscription, error) {
	rows, err := r.db.Query(`
		SELECT id, email, repo, confirmed, confirm_token, unsubscribe_token, last_seen_tag
		FROM subscriptions WHERE email = $1 AND confirmed = true`, email)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("failed to close db response: %v", err)
		}
	}()

	var subs []Subscription
	for rows.Next() {
		var s Subscription
		if err := rows.Scan(&s.ID, &s.Email, &s.Repo, &s.Confirmed, &s.ConfirmToken, &s.UnsubscribeToken, &s.LastSeenTag); err != nil {
			return nil, err
		}
		subs = append(subs, s)
	}
	return subs, nil
}

func (r *SubscriptionRepo) FindAllConfirmed() ([]Subscription, error) {
	rows, err := r.db.Query(`
		SELECT id, email, repo, confirmed, confirm_token, unsubscribe_token, last_seen_tag
		FROM subscriptions WHERE confirmed = true`)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("failed to close db response: %v", err)
		}
	}()

	var subs []Subscription
	for rows.Next() {
		var s Subscription
		if err := rows.Scan(&s.ID, &s.Email, &s.Repo, &s.Confirmed, &s.ConfirmToken, &s.UnsubscribeToken, &s.LastSeenTag); err != nil {
			return nil, err
		}
		subs = append(subs, s)
	}
	return subs, nil
}

func (r *SubscriptionRepo) UpdateLastSeenTag(id int, tag string) error {
	_, err := r.db.Exec(
		`UPDATE subscriptions SET last_seen_tag = $1 WHERE id = $2`, tag, id)
	return err
}
