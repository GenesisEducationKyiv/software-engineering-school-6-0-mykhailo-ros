package github_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github-release-notifier/internal/github"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- mock cache ---

type mockCache struct {
	data   map[string]string
	getErr error
	setErr error
	setCalls int
}

func newMockCache() *mockCache {
	return &mockCache{data: make(map[string]string)}
}

func (m *mockCache) Get(key string) (string, error) {
	if m.getErr != nil {
		return "", m.getErr
	}
	v, ok := m.data[key]
	if !ok {
		return "", errors.New("cache miss")
	}
	return v, nil
}

func (m *mockCache) Set(key, value string, _ time.Duration) error {
	m.setCalls++
	if m.setErr != nil {
		return m.setErr
	}
	m.data[key] = value
	return nil
}

// --- helpers ---

func stubbedClient(t *testing.T, handler http.Handler) (*github.Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return github.NewTestClient("", srv.URL, nil), srv
}

func stubbedClientWithCache(t *testing.T, handler http.Handler, c *mockCache) *github.Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return github.NewTestClient("", srv.URL, c)
}

// --- RepoExists ---

func TestRepoExists_Valid(t *testing.T) {
	client, _ := stubbedClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	got, err := client.RepoExists("owner/repo")
	require.NoError(t, err)
	assert.True(t, got)
}

func TestRepoExists_NotFound(t *testing.T) {
	client, _ := stubbedClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	got, err := client.RepoExists("owner/repo")
	require.NoError(t, err)
	assert.False(t, got)
}

func TestRepoExists_RateLimited(t *testing.T) {
	client, _ := stubbedClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	got, err := client.RepoExists("owner/repo")
	assert.False(t, got)
	assert.Error(t, err)
}

func TestRepoExists_UnexpectedStatus(t *testing.T) {
	client, _ := stubbedClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	got, err := client.RepoExists("owner/repo")
	assert.False(t, got)
	assert.Error(t, err)
}

func TestRepoExists_InvalidFormat(t *testing.T) {
	client := github.NewTestClient("", "http://stub", nil)
	_, err := client.RepoExists("noslash")
	assert.Error(t, err)
}

func TestRepoExists_EmptyParts(t *testing.T) {
	client := github.NewTestClient("", "http://stub", nil)
	_, err := client.RepoExists("/repo")
	assert.Error(t, err)
}

// --- GetLatestRelease ---

func TestGetLatestRelease_CacheHit(t *testing.T) {
	cache := newMockCache()
	cache.data["release:owner/repo"] = "v3.0.0"

	// handler should never be called
	called := false
	client := stubbedClientWithCache(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}), cache)

	release, err := client.GetLatestRelease("owner/repo")
	require.NoError(t, err)
	assert.Equal(t, "v3.0.0", release.TagName)
	assert.False(t, called, "HTTP should not be called on cache hit")
}

func TestGetLatestRelease_CacheMiss_Success(t *testing.T) {
	cache := newMockCache()

	client := stubbedClientWithCache(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"tag_name": "v1.2.3"})
	}), cache)

	release, err := client.GetLatestRelease("owner/repo")
	require.NoError(t, err)
	assert.Equal(t, "v1.2.3", release.TagName)
	assert.Equal(t, "v1.2.3", cache.data["release:owner/repo"], "should populate cache")
}

func TestGetLatestRelease_NotFound(t *testing.T) {
	client, _ := stubbedClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	release, err := client.GetLatestRelease("owner/repo")
	require.NoError(t, err)
	assert.Equal(t, "", release.TagName, "empty release when no releases published")
}

func TestGetLatestRelease_RateLimited(t *testing.T) {
	client, _ := stubbedClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	_, err := client.GetLatestRelease("owner/repo")
	assert.Error(t, err)
}

func TestGetLatestRelease_EmptyTagNotCached(t *testing.T) {
	cache := newMockCache()

	client := stubbedClientWithCache(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"tag_name": ""})
	}), cache)

	_, err := client.GetLatestRelease("owner/repo")
	require.NoError(t, err)
	assert.Equal(t, 0, cache.setCalls, "empty tag must not be written to cache")
}

func TestGetLatestRelease_NilCache(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"tag_name": "v9.0.0"})
	}))
	t.Cleanup(srv.Close)
	client := github.NewTestClient("", srv.URL, nil)

	release, err := client.GetLatestRelease("owner/repo")
	require.NoError(t, err)
	assert.Equal(t, "v9.0.0", release.TagName)
}
