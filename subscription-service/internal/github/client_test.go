package github_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"subscription-service/internal/github"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- helpers ---

func stubbedClient(t *testing.T, handler http.Handler) (*github.Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return github.NewTestClient("", srv.URL), srv
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
	client := github.NewTestClient("", "http://stub")
	_, err := client.RepoExists("noslash")
	assert.Error(t, err)
}

func TestRepoExists_EmptyParts(t *testing.T) {
	client := github.NewTestClient("", "http://stub")
	_, err := client.RepoExists("/repo")
	assert.Error(t, err)
}

// --- GetLatestRelease ---

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

func TestGetLatestRelease_Success(t *testing.T) {
	client, _ := stubbedClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"tag_name": "v9.0.0"})
	}))
	release, err := client.GetLatestRelease("owner/repo")
	require.NoError(t, err)
	assert.Equal(t, "v9.0.0", release.TagName)
}
