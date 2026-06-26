# ADR 008: Redis Caching as a Decorator — Non-Critical Dependency

**Status:** Accepted
**Date:** 2026-06-26
**Supersedes:** [ADR 003](003-redis-caching.md)

## Context

ADR 003 decided to use Redis as a 10-minute TTL cache to reduce GitHub API calls. The original implementation embedded the cache logic directly inside `github.Client.GetLatestRelease`. Two problems emerged:

1. **Wrong layer**: cache logic inside the HTTP client violated SRP. The client handled both transport concerns and caching policy. Swapping or disabling the cache required editing the client.
2. **Hard dependency**: a `cache.Set` failure caused `GetLatestRelease` to return an error, making Redis a hard runtime requirement. A Redis outage would stop release notifications entirely even though GitHub was reachable.

## Decision

Caching is extracted into a **decorator**, `CachingReleaseChecker` (`internal/github/caching.go`), that wraps any `ReleaseGetter`:

```go
type ReleaseGetter interface {
    GetLatestRelease(repo string) (*domain.Release, error)
}

type CachingReleaseChecker struct {
    inner ReleaseGetter
    cache Cache
    ttl   time.Duration
}
```

`github.Client` retains only the HTTP fetch; all cache logic lives in the decorator. Composition happens in `main.go`:

```go
rawGithub    := github.NewClient(cfg.GithubToken)
cachedGithub := github.NewCachingReleaseChecker(rawGithub, cacheClient, 10*time.Minute)
```

The scheduler receives `cachedGithub`; the service (which only calls `RepoExists`) receives `rawGithub` directly.

On a `cache.Set` failure the error is **logged and discarded** — the release is still returned to the caller. Redis is non-critical: an outage causes cache misses (more GitHub API calls) but does not prevent notifications.

## Consequences

- Redis is no longer a hard runtime dependency. The app degrades gracefully if Redis is unavailable.
- Cache and HTTP transport concerns are separated. Disabling or replacing the cache requires only a change in `main.go`.
- The `ReleaseGetter` interface makes `CachingReleaseChecker` independently testable without a real HTTP client or Redis instance.
- `cache.Get` misses and `cache.Set` errors both appear in logs, giving operators visibility into cache health without alerting on them.
