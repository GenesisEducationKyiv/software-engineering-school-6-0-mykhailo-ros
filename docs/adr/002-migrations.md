# ADR 002: Database Migrations — golang-migrate, Run on Startup

**Status:** Accepted
**Date:** 2025-04-12


## Context

The task required database schema migrations. A migration tool and execution strategy had to be chosen.

## Decision

`golang-migrate` was chosen. Migrations are plain SQL files and run automatically when the application starts, before the HTTP server begins accepting requests.

## Reasons

- **Plain SQL**: migrations are written in raw SQL (`000001_init.up.sql`, `000001_init.down.sql`), not in a Go DSL. This keeps them readable and portable.
- **Up/down pairs**: each migration has a corresponding rollback file, making it straightforward to undo schema changes during development.
- **Startup execution**: running migrations at startup means the schema is always in sync with the binary being deployed. No manual migration step is required.
- **`golang-migrate` fit**: the library integrates with `database/sql` and supports the `file://` source for local SQL files, which matches the project's layout.

## Consequences

- Migrations run on every startup, including in production. `golang-migrate` is idempotent — it tracks applied migrations in a `schema_migrations` table and skips already-applied ones.
- If a migration fails at startup, the application exits rather than starting with an inconsistent schema.
