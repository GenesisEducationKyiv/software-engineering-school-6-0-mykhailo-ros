# Testing Guide

## Prerequisites

- [Git](https://git-scm.com/)
- [Go](https://go.dev/) (version from `go.mod`)
- [Docker](https://docs.docker.com/get-docker/) with the Compose plugin

No other tools are required.

---

## Run everything

```sh
make test-unit
make test-integration
make test-e2e
```

---

## Unit tests

No Docker required.

```sh
make test-unit
# expands to: go test ./...
```

Integration test files are excluded automatically via their `//go:build integration` tag.

---

## Integration tests

Requires the Docker daemon to be running (Docker Desktop or Docker Engine).
[testcontainers-go](https://testcontainers.com/guides/getting-started-with-testcontainers-for-go/)
spins up isolated `postgres:16-alpine` and `redis:7-alpine` containers
automatically and removes them when the tests finish.

```sh
make test-integration
# expands to: go test -tags=integration -timeout=120s ./internal/integration/...
```

---

## E2E tests

Requires the Docker daemon to be running. `make test-e2e` brings up the
entire application stack from scratch — Postgres, Redis, MailHog, the app,
and a Playwright container — then tears it all down when done. No containers
need to be running beforehand.

```sh
make test-e2e
```

This command:
1. Builds and starts all services including the `e2e` container
2. Waits for the app to pass its health check before running tests
3. Exits with Playwright's exit code
4. Tears everything down

### View the HTML report

The report is written to `e2e/playwright-report/` on the host (via volume
mount). Open `e2e/playwright-report/index.html` directly in a browser.

```sh
xdg-open e2e/playwright-report/index.html
```

---

## Inspecting emails (MailHog)

The `docker-compose.yml` includes [MailHog](https://github.com/mailhog/MailHog)
as an SMTP sink. All outgoing emails are captured there — no real email is sent.

Open the MailHog web UI at: <http://localhost:8025>
