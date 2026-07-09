# ADR 009: Scheduler — time.Ticker with Context-Based Graceful Shutdown

**Status:** Accepted
**Date:** 2026-06-26
**Supersedes:** [ADR 004](004-scheduler-design.md)

## Context

ADR 004 chose a goroutine with `time.Sleep` for the scheduler. The core reasons — simplicity, no external dependencies, sufficient precision at a 10-minute interval — remain valid. However, the decision did not account for graceful shutdown: `time.Sleep` has no cancellation path. When the process receives SIGINT/SIGTERM, the scheduler goroutine either runs through the full sleep or is killed abruptly mid-tick.

Adding context-based graceful shutdown to the HTTP server (via `http.Server.Shutdown`) exposed this gap — the server would drain in-flight HTTP requests cleanly, but the scheduler goroutine had no equivalent mechanism.

## Decision

Replace `time.Sleep` with `time.NewTicker` and a `select` on both the ticker channel and the context done channel:

```go
func (s *Scheduler) Start(ctx context.Context) {
    go func() {
        ticker := time.NewTicker(s.interval)
        defer ticker.Stop()
        for {
            select {
            case <-ticker.C:
                s.check()
            case <-ctx.Done():
                return
            }
        }
    }()
}
```

`main.go` derives the context from `signal.NotifyContext` so both the HTTP server and the scheduler respond to the same OS signal:

```go
ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
defer stop()
scheduler.Start(ctx)
```

## Reasons

- **Graceful shutdown**: the goroutine exits as soon as the context is cancelled, regardless of where it is in the sleep cycle.
- **`time.Ticker` fires at fixed wall-clock intervals**, unlike `time.Sleep` which drifts by the duration of each tick. At a 10-minute interval the drift is negligible, but `Ticker` is the more correct primitive when interval regularity is the intent.
- The change is minimal and introduces no new dependencies.

## Consequences

- The scheduler goroutine stops promptly on shutdown instead of waiting up to 10 minutes for the current sleep to expire.
- `ticker.Stop()` via `defer` prevents a goroutine and channel leak on exit.
- The remaining consequences from ADR 004 are unchanged: scheduler and HTTP server share the same process; horizontal scaling causes duplicate polling; sequential iteration per tick is sufficient at current scale.
