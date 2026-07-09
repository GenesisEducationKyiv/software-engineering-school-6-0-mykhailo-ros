# ADR 001: Database — PostgreSQL

**Status:** Accepted
**Date:** 2025-04-12


## Context

The service needs persistent storage for subscriptions, tokens, and release tracking state. A database had to be chosen.

## Options Considered

**PostgreSQL**

Pros:
- ACID transactions — reliable concurrent writes from the scheduler and HTTP handlers
- `UNIQUE` constraints enforce business rules at the DB level (one subscription per email+repo)
- Rich type system (`SERIAL`, `TIMESTAMPTZ`) — no workarounds needed for the schema
- Official Docker image integrates cleanly with Docker Compose
- Compatible with Go's `database/sql` via `lib/pq`

Cons:
- Requires a running server instance; more overhead than an embedded DB

**SQLite**

Pros:
- Zero setup — file-based, no separate process

Cons:
- Weak concurrent write support — problematic when the scheduler and HTTP handlers write simultaneously
- Not suitable for production deployments

## Decision

PostgreSQL was chosen.

## Consequences

- Requires a running PostgreSQL instance; handled via Docker Compose.
- Migrations must be managed explicitly — addressed by ADR 002.
