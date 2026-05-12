# System Design: GitHub Release Notifier

## Overview

GitHub Release Notifier is a monolithic REST API service that allows users to subscribe via email to release notifications for GitHub repositories. When a new release is published, subscribers receive an email notification.

## Architecture

The service follows a layered monolithic architecture:

```
┌─────────────────────────────────────────────────┐
│                   HTTP Layer (Gin)              │
│                  internal/handler               │
└────────────────────────┬────────────────────────┘
                         │
┌────────────────────────▼────────────────────────┐
│               Business Logic Layer              │
│                  internal/service               │
└──────┬──────────────┬──────────────┬────────────┘
       │              │              │
┌──────▼──────┐ ┌─────▼──────┐ ┌─────▼───────────┐
│  Repository │ │   GitHub   │ │     Mailer      │
│    Layer    │ │   Client   │ │  internal/mailer│
│internal/repo│ │internal/gh │ └─────────────────┘
└──────┬──────┘ └─────┬──────┘
       │              │
┌──────▼──────┐ ┌─────▼──────┐
│ PostgreSQL  │ │   Redis    │
│  Database   │ │   Cache    │
└─────────────┘ └────────────┘

┌─────────────────────────────────────────────────┐
│              Background Scheduler               │
│              internal/scheduler                 │
│   (polls GitHub API, triggers notifications)    │
└─────────────────────────────────────────────────┘
```

## Components

### Handler (`internal/handler`)
Accepts HTTP requests, validates input, delegates to the service layer, and maps results to HTTP responses. No business logic lives here.

### Service (`internal/service`)
Contains all business logic: subscription lifecycle management, token generation, duplicate detection, and orchestration of the repository, GitHub client, and mailer.

### Repository (`internal/repository`)
Handles all database access via raw SQL. Isolated behind an interface so it can be swapped or mocked in tests.

### GitHub Client (`internal/github`)
Wraps the GitHub REST API. Handles 429 rate limiting with backoff, and caches responses in Redis with a 10-minute TTL to stay within the 60 req/hour unauthenticated limit.

### Mailer (`internal/mailer`)
Sends transactional emails via SMTP. Used for confirmation and unsubscribe links.

### Scheduler (`internal/scheduler`)
A background goroutine that iterates all confirmed subscriptions every 10 minutes, checks GitHub for new releases, and triggers email notifications when `last_seen_tag` changes.

## Data Flow

### Subscribe
```
POST /api/subscribe (email, repo)
  → validate format
  → check repo exists via GitHub API
  → create subscription (confirmed=false, generate tokens)
  → send confirmation email
  → 200 OK
```

### Confirm
```
GET /api/confirm/:token
  → look up subscription by confirm_token
  → set confirmed=true
  → 200 OK
```

### Notify (background)
```
Scheduler tick (every 10 minutes)
  → load all confirmed subscriptions
  → for each: fetch latest release from GitHub
  → if release tag != last_seen_tag:
      → send notification email
      → update last_seen_tag
```

### Unsubscribe
```
GET /api/unsubscribe/:token
  → look up subscription by unsubscribe_token
  → delete subscription
  → 200 OK
```

## Database Schema

```sql
CREATE TABLE subscriptions (
    id                 SERIAL PRIMARY KEY,
    email              TEXT NOT NULL,
    repo               TEXT NOT NULL,
    confirmed          BOOLEAN NOT NULL DEFAULT FALSE,
    confirm_token      TEXT NOT NULL UNIQUE,
    unsubscribe_token  TEXT NOT NULL UNIQUE,
    last_seen_tag      TEXT NOT NULL DEFAULT '',
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(email, repo)
);
```

The `UNIQUE(email, repo)` constraint enforces that a user cannot subscribe to the same repository twice and maps to a 409 Conflict response.

## External Dependencies

| Dependency    | Purpose                              | Notes                                  |
|---------------|--------------------------------------|----------------------------------------|
| GitHub API    | Verify repos, fetch latest releases  | 60 req/hr unauth, 5000 with token      |
| SMTP server   | Send confirmation and notification emails | Mailtrap used in development       |
| PostgreSQL    | Persistent storage of subscriptions  | Migrations run on startup              |
| Redis         | Cache GitHub API responses           | TTL: 10 minutes                        |

## Deployment

The entire system runs via Docker Compose:

```
docker-compose up
```

Services:
- `app` — the Go binary
- `db` — PostgreSQL
- `redis` — Redis

The app container waits for the database to be ready, then runs migrations automatically before starting the HTTP server.

## Known Limitations

- **Single instance only**: the scheduler runs as a goroutine inside the app process. Horizontal scaling would require distributed locking or moving the scheduler to a separate worker.
- **No retry on failed emails**: if SMTP fails during notification, the `last_seen_tag` is not updated and the next scheduler tick will retry — but within the same tick, there is no retry logic.
- **No Redis fallback**: if Redis is unavailable, GitHub API calls will fail rather than fall through to a live request. Redis must be healthy for the service to function.
- **Unauth GitHub rate limit**: without a token configured, the service is limited to 60 GitHub API requests per hour across all subscriptions.