package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github-release-notifier/internal/domain"
)

var errLatestReleaseNotFound = errors.New("latest release not found")

type Client struct {
	httpClient *http.Client
	token      string
	apiBase    string
}

func NewClient(token string) *Client {
	if token == "" {
		log.Println("github: no token configured, unauthenticated rate limit is 60 req/hour")
	}
	return &Client{
		httpClient: &http.Client{},
		token:      token,
		apiBase:    "https://api.github.com",
	}
}

func NewTestClient(token, apiBase string) *Client {
	return &Client{
		httpClient: &http.Client{},
		token:      token,
		apiBase:    apiBase,
	}
}

func (c *Client) sendRequest(url string) (*http.Response, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	return c.httpClient.Do(req)
}

func checkResponseStatus(resp *http.Response) error {
	switch resp.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusNotFound:
		return errLatestReleaseNotFound
	case http.StatusTooManyRequests:
		return domain.ErrRateLimited
	default:
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}
}

func (c *Client) RepoExists(repo string) (bool, error) {
	parts := strings.Split(repo, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return false, fmt.Errorf("invalid repo format")
	}

	resp, err := c.sendRequest(fmt.Sprintf("%s/repos/%s", c.apiBase, repo))
	if err != nil {
		return false, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("failed to close response body: %v", err)
		}
	}()

	if err := checkResponseStatus(resp); err != nil {
		if errors.Is(err, errLatestReleaseNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (c *Client) GetLatestRelease(repo string) (*domain.Release, error) {
	resp, err := c.sendRequest(fmt.Sprintf("%s/repos/%s/releases/latest", c.apiBase, repo))
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("failed to close response body: %v", err)
		}
	}()

	if err := checkResponseStatus(resp); err != nil {
		if errors.Is(err, errLatestReleaseNotFound) {
			return &domain.Release{}, nil
		}
		return nil, err
	}

	var release domain.Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}

	return &release, nil
}
