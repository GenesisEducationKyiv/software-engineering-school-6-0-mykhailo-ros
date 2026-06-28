package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"notification-service/internal/domain"
)

type SubscriptionClient struct {
	baseURL string
	http    *http.Client
}

func NewSubscriptionClient(baseURL string) *SubscriptionClient {
	return &SubscriptionClient{baseURL: baseURL, http: &http.Client{}}
}

func (c *SubscriptionClient) FindAllConfirmed() ([]domain.Subscription, error) {
	resp, err := c.http.Get(c.baseURL + "/internal/subscriptions")
	if err != nil {
		return nil, fmt.Errorf("client: list confirmed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("client: list confirmed: unexpected status %d", resp.StatusCode)
	}

	var items []struct {
		ID          int    `json:"id"`
		Email       string `json:"email"`
		Repo        string `json:"repo"`
		LastSeenTag string `json:"last_seen_tag"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, fmt.Errorf("client: list confirmed: decode: %w", err)
	}

	subs := make([]domain.Subscription, len(items))
	for i, it := range items {
		subs[i] = domain.Subscription{ID: it.ID, Email: it.Email, Repo: it.Repo, LastSeenTag: it.LastSeenTag}
	}
	return subs, nil
}

func (c *SubscriptionClient) UpdateLastSeenTag(id int, tag string) error {
	body, _ := json.Marshal(map[string]string{"tag": tag})
	req, err := http.NewRequest(http.MethodPatch,
		fmt.Sprintf("%s/internal/subscriptions/%d/last-seen-tag", c.baseURL, id),
		bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("client: update tag: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("client: update tag: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("client: update tag: unexpected status %d", resp.StatusCode)
	}
	return nil
}
