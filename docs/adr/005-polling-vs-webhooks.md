# ADR 005: GitHub Integration Strategy — Polling over Webhooks

**Status:** Accepted
**Date:** 2025-04-12


## Context

The service needs to detect new GitHub releases. GitHub offers two ways to receive this information: polling the REST API on a schedule, or registering webhooks that push events to the service in real time.

## Options Considered

**Polling (GitHub REST API)**

Pros:
- No public endpoint required — the service does not need to be reachable from the internet
- No webhook lifecycle management — no need to register/deregister hooks per subscription
- Resilient to downtime — if the service is down, the next tick catches up automatically
- Simple failure model — one code path handles all repos uniformly

Cons:
- Inherent delay of up to 10 minutes between a release and notification
- Subject to GitHub API rate limits

**Webhooks (GitHub push events)**

Pros:
- Real-time — notification triggered immediately when a release is published
- No polling overhead — zero API calls when nothing changes

Cons:
- Requires a publicly reachable HTTPS endpoint — unavailable in the current Docker Compose deployment
- Requires registering a webhook on GitHub per subscription and deregistering on unsubscribe
- Events delivered while the service is down are lost unless GitHub retries and the service handles duplicates

## Decision

Polling via the GitHub REST API was chosen.

## Consequences

- Notifications are delayed by up to 10 minutes after a release is published. Acceptable for an email notification service.
- GitHub API rate limits impose a ceiling on unique repos trackable per hour — mitigated by Redis caching (ADR 003) and a GitHub token.
- The service is not suitable for use cases requiring real-time release detection.
