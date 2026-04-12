package github

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github-release-notifier/internal/cache"
)

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

func (c *Client) RepoExists(repo string) (bool, error) {
	parts := strings.Split(repo, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return false, fmt.Errorf("invalid repo format")
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s", repo)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false, err
	}

	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return false, fmt.Errorf("github rate limit exceeded")
	}
	return resp.StatusCode == http.StatusOK, nil
}

func (c *Client) GetLatestRelease(repo string) (*Release, error) {
	cacheKey := "release:" + repo

	if c.cache != nil {
		if cached, err := c.cache.Get(cacheKey); err == nil {
			return &Release{TagName: cached}, nil
		}
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("github rate limit exceeded")
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}

	if c.cache != nil && release.TagName != "" {
		c.cache.Set(cacheKey, release.TagName, 10*time.Minute)
	}

	return &release, nil
}
