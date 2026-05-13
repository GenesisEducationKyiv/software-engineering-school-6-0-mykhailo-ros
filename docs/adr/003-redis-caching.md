# ADR 003: Redis Caching of GitHub API Responses

**Status:** Accepted
**Date:** 2025-04-12


## Context

The GitHub API enforces a rate limit of 60 requests per hour for unauthenticated clients. The scheduler polls GitHub for every active subscription on each tick. Without caching, a modest number of subscriptions can exhaust this limit quickly.

## Options Considered

**Redis cache (external, TTL-based)**

Pros:
- Shared across all subscribers — one cache hit serves all users subscribed to the same repo
- Survives app restarts — the Redis container is separate from the app container
- TTL-based invalidation is simple and fits the use case

Cons:
- Adds Redis as a required infrastructure dependency
- No fallback if Redis is unavailable — hard failure

**In-memory map inside the Go process**

Pros:
- Zero infrastructure overhead

Cons:
- Cleared on every app restart — rate limit budget wasted on re-fetching known data

**No caching**

Pros:
- Simplest implementation

Cons:
- Each subscription triggers a GitHub API call per tick — exhausts the 60 req/hr limit with as few as 10 unique repos

## Decision

Redis cache with a 10-minute TTL was chosen. Caching is applied at the GitHub client layer (`internal/github`), transparent to the service and scheduler.

## Consequences

- Adds Redis as a required infrastructure dependency.
- Cache invalidation is TTL-based only — no mechanism to force a refresh. Acceptable for this use case.
- If Redis is unavailable, GitHub API calls fail. Redis is a hard dependency for the service to operate.
