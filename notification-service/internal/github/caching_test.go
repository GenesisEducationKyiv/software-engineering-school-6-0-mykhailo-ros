package github_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"notification-service/internal/domain"
	"notification-service/internal/github"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- mocks ---

type mockCache struct {
	data     map[string]string
	getErr   error
	setErr   error
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

// stubbedChecker wraps an httptest.Server as a ReleaseGetter via a real Client.
func stubbedChecker(t *testing.T, handler http.Handler) (github.ReleaseGetter, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return github.NewTestClient("", srv.URL), srv
}

// --- CachingReleaseChecker tests ---

func TestCachingChecker_CacheHit(t *testing.T) {
	cache := newMockCache()
	cache.data["release:owner/repo"] = "v3.0.0"

	called := false
	inner, _ := stubbedChecker(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	checker := github.NewCachingReleaseChecker(inner, cache, 10*time.Minute)
	release, err := checker.GetLatestRelease("owner/repo")
	require.NoError(t, err)
	assert.Equal(t, "v3.0.0", release.TagName)
	assert.False(t, called, "HTTP must not be called on cache hit")
}

func TestCachingChecker_CacheMiss_PopulatesCache(t *testing.T) {
	cache := newMockCache()
	inner := &mockReleaseGetter{tag: "v1.2.3"}

	checker := github.NewCachingReleaseChecker(inner, cache, 10*time.Minute)
	release, err := checker.GetLatestRelease("owner/repo")
	require.NoError(t, err)
	assert.Equal(t, "v1.2.3", release.TagName)
	assert.Equal(t, "v1.2.3", cache.data["release:owner/repo"])
}

func TestCachingChecker_EmptyTagIsCached(t *testing.T) {
	cache := newMockCache()
	inner := &mockReleaseGetter{tag: ""}

	checker := github.NewCachingReleaseChecker(inner, cache, 10*time.Minute)
	_, err := checker.GetLatestRelease("owner/repo")
	require.NoError(t, err)
	assert.Equal(t, 1, cache.setCalls, "empty tag (repo with no releases) must still be cached to avoid repeated GitHub calls")
}

func TestCachingChecker_SetError_DoesNotFail(t *testing.T) {
	cache := newMockCache()
	cache.setErr = errors.New("redis down")
	inner := &mockReleaseGetter{tag: "v2.0.0"}

	checker := github.NewCachingReleaseChecker(inner, cache, 10*time.Minute)
	release, err := checker.GetLatestRelease("owner/repo")
	require.NoError(t, err, "cache write failure must not propagate as an error")
	assert.Equal(t, "v2.0.0", release.TagName)
}

func TestCachingChecker_InnerError_Propagates(t *testing.T) {
	cache := newMockCache()
	inner := &mockReleaseGetter{err: errors.New("rate limited")}

	checker := github.NewCachingReleaseChecker(inner, cache, 10*time.Minute)
	_, err := checker.GetLatestRelease("owner/repo")
	assert.Error(t, err)
}

// mockReleaseGetter is a simple in-memory stand-in for github.Client.
type mockReleaseGetter struct {
	tag string
	err error
}

func (m *mockReleaseGetter) GetLatestRelease(_ string) (*domain.Release, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &domain.Release{TagName: m.tag}, nil
}
