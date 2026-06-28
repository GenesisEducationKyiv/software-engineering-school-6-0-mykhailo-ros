package github

import (
	"log/slog"
	"time"

	"notification-service/internal/domain"
)

type Cache interface {
	Get(key string) (string, error)
	Set(key, value string, ttl time.Duration) error
}

type ReleaseGetter interface {
	GetLatestRelease(repo string) (*domain.Release, error)
}

type CachingReleaseChecker struct {
	inner ReleaseGetter
	cache Cache
	ttl   time.Duration
}

func NewCachingReleaseChecker(inner ReleaseGetter, cache Cache, ttl time.Duration) *CachingReleaseChecker {
	return &CachingReleaseChecker{inner: inner, cache: cache, ttl: ttl}
}

func (c *CachingReleaseChecker) GetLatestRelease(repo string) (*domain.Release, error) {
	key := "release:" + repo

	if val, err := c.cache.Get(key); err == nil {
		return &domain.Release{TagName: val}, nil
	}

	release, err := c.inner.GetLatestRelease(repo)
	if err != nil {
		return nil, err
	}

	if release.TagName != "" {
		if err := c.cache.Set(key, release.TagName, c.ttl); err != nil {
			slog.Error("cache: failed to write release", "repo", repo, "error", err)
		}
	}

	return release, nil
}
