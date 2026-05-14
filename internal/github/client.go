package github

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github-release-notifier/internal/cache"
)

var ErrRepoNotFound = errors.New("repository not found")

type Client struct {
	httpClient *http.Client
	token      string
	cache      *cache.Cache
}

func NewClient(token string, cache *cache.Cache) *Client {
	return &Client{
		httpClient: &http.Client{},
		token:      token,
		cache:      cache,
	}
}

type Release struct {
	TagName string `json:"tag_name"`
}

func (c *Client) sendRequest(url string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
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
		return ErrRepoNotFound
	case http.StatusTooManyRequests:
		return fmt.Errorf("github rate limit exceeded")
	default:
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}
}

func (c *Client) RepoExists(repo string) (bool, error) {
	parts := strings.Split(repo, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return false, fmt.Errorf("invalid repo format")
	}

	resp, err := c.sendRequest(fmt.Sprintf("https://api.github.com/repos/%s", repo))
	if err != nil {
		return false, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("failed to close response body: %v", err)
		}
	}()

	if err := checkResponseStatus(resp); err != nil {
		if errors.Is(err, ErrRepoNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (c *Client) GetLatestRelease(repo string) (*Release, error) {
	cacheKey := "release:" + repo

	if c.cache != nil {
		if cached, err := c.cache.Get(cacheKey); err == nil {
			return &Release{TagName: cached}, nil
		}
	}

	resp, err := c.sendRequest(fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo))
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("failed to close response body: %v", err)
		}
	}()

	if err := checkResponseStatus(resp); err != nil {
		return nil, err
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}

	if c.cache != nil && release.TagName != "" {
		err = c.cache.Set(cacheKey, release.TagName, 10*time.Minute)
		if err != nil {
			return nil, err
		}
	}

	return &release, nil
}
