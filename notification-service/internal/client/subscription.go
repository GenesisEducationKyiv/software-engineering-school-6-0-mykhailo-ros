package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"notification-service/internal/domain"
	"time"
)

type RESTClient struct {
	baseURL       string
	internalToken string
	http          *http.Client
}

func NewRESTClient(baseURL, internalToken string) *RESTClient {
	return &RESTClient{
		baseURL:       baseURL,
		internalToken: internalToken,
		http:          &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *RESTClient) FindAllConfirmed(ctx context.Context) ([]domain.Subscription, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/internal/subscriptions", nil)
	if err != nil {
		return nil, fmt.Errorf("client: list confirmed: %w", err)
	}
	req.Header.Set("X-Internal-Token", c.internalToken)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("client: list confirmed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

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

func (c *RESTClient) UpdateLastSeenTag(ctx context.Context, id int, tag string) error {
	body, err := json.Marshal(map[string]string{"tag": tag})
	if err != nil {
		return fmt.Errorf("client: update tag: marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPatch,
		fmt.Sprintf("%s/internal/subscriptions/%d/last-seen-tag", c.baseURL, id),
		bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("client: update tag: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Token", c.internalToken)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("client: update tag: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("client: update tag: unexpected status %d", resp.StatusCode)
	}
	return nil
}
