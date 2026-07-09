package github

import (
	"log"
	"time"

	"github-release-notifier/internal/domain"
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
			log.Printf("cache: failed to write release for %s: %v", repo, err)
		}
	}

	return release, nil
}
