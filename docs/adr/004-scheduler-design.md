# ADR 004: Background Scheduler — Goroutine with time.Sleep

**Status:** Accepted
**Date:** 2025-04-12


## Context

The service must periodically check all active subscriptions for new GitHub releases and send email notifications. This requires a recurring background process running alongside the HTTP server.

## Decision

The scheduler is implemented as a goroutine using a `for {}` loop with `time.Sleep`, started at application startup in `main.go` alongside the HTTP server.

## Reasons

- **Simplicity**: a goroutine with a sleep loop requires no external dependencies and no separate process. Go's concurrency model makes this natural.
- **Monolith requirement**: the task explicitly requires all functionality to run within a single service. An external cron job or message queue would violate this constraint.
- **Sufficient for scope**: the polling interval does not need sub-second precision. Sleeping 10 minutes between ticks is adequate for an email notification service.

`time.Sleep` was chosen over `time.Ticker` for simplicity. The tradeoff is that Sleep waits *after* each tick completes, so the effective interval drifts by however long processing takes. `time.Ticker` fires at fixed wall-clock intervals regardless of processing time. At a 10-minute interval and low subscription counts, this drift is negligible.

Alternatives considered:

- **External cron**: simpler to reason about, but requires a separate process and doesn't fit the monolith constraint.
- **Database-backed job queue**: more robust (survives restarts mid-tick), but far more complex than warranted here.

## Consequences

- The scheduler and HTTP server share the same process. A crash in either affects both.
- Horizontal scaling would cause multiple instances to poll simultaneously — this is a known limitation (see system-design.md). For the current scope of a single-instance deployment, it is not a problem.
- The scheduler iterates subscriptions sequentially per tick. If the number of subscriptions grows large, this could cause a tick to take longer than the interval. A worker pool could address this if needed.
