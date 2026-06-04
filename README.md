# GitHub Release Notification API

### GitHub Release Notification API that allows users to subscribe to email notifications about new releases of a chosen GitHub repository.

by Mykhailo Rosliakov as a part of a Test Case for Software Engineering School 6.0 by Genesis.

## 1. Functionality

The functionality of this monolith API is simple and it was designed for one purpose: to notify a user on every new release of a github repository that they have signed up to. 

### Completed Requirements

**Mandatory:**
- REST API with all 4 endpoints matching the Swagger specification
- PostgreSQL database with automatic migrations on startup
- Email confirmation flow with unique tokens
- Scheduler checks GitHub releases every 10 minutes, notifies on new releases
- GitHub repository validation on subscription with correct 400/404 responses
- GitHub API rate limit (429) handling
- Dockerfile and docker-compose.yml for full Docker deployment
- Unit tests for service and handler layers

**Extra:**
- Redis caching of GitHub API responses with 10 minute TTL
- Swagger UI at `/swagger`
- GitHub Actions CI pipeline

## 2. Architecture

The whole functionality was divided between 6 layers and a background process scheduler that is responsible for running core functions every 10 minutes. 

#### The Layers are:
- `Handler` - manages http requests.
- `Service` - core business logic. defines the main functionality and ties everything else together. 
- `Repository` - DB interaction.
- `GitHub client ` - GitHub API interaction.
- `Mailer` - SMTP managing.
- `Cache` - Redis wrapper.

Here is a representation of what is happening under the hood every time a user hits an endpoint:

User hits endpoint → Handler parses the HTTP request, validates input, calls Service → Service contains business logic, calls Repository for DB operations, GitHub client to verify repos and check releases, Mailer to send emails → Repository runs SQL queries against PostgreSQL → GitHub client checks Redis Cache first before hitting the GitHub API → results flow back up the same chain → Handler writes the HTTP response.

Scheduler runs in the background the whole time and checks the release of every unique repo every 10 minutes, in case of a new release sends a notification to subscribed users.

## 3. How to run

#### **To start the service**:
1. Set up environment variables in .env

2. Run `docker-compose up --build`


Everything needed is managed by docker (including PostgreSQL and Redis) 

## 4. Environment Variables

| Variable | Description |
|---|---|
| `DB_HOST` | PostgreSQL host (`db` for Docker, `localhost` for local) |
| `DB_PORT` | PostgreSQL port (default: `5432`) |
| `DB_USER` | PostgreSQL user |
| `DB_PASSWORD` | PostgreSQL password |
| `DB_NAME` | PostgreSQL database name |
| `SMTP_HOST` | SMTP server host |
| `SMTP_PORT` | SMTP server port |
| `SMTP_USERNAME` | SMTP username |
| `SMTP_PASSWORD` | SMTP password |
| `SMTP_FROM` | Sender email address |
| `BASE_URL` | Public base URL of the service (e.g. `http://localhost:8080`) |
| `GITHUB_TOKEN` | GitHub personal access token (optional, increases rate limit from 60 to 5000 req/hour) |
| `REDIS_ADDR` | Redis address (`redis:6379` for Docker, `localhost:6379` for local) |

## 5. API

There is a swagger.yaml documentation describing details of each endpoint. 

Additionally there is `/swagger` endpoint that uses Swagger UI to ease testing.

### Short endpoints description:

| Method | Endpoint | Description |
|---|---|---|
| `POST` | `/api/subscribe` | Subscribe an email to release notifications for a given GitHub repository |
| `GET` | `/api/confirm/{token}` | Confirm email subscription using the token sent in the confirmation email |
| `GET` | `/api/unsubscribe/{token}` | Unsubscribe from release notifications |
| `GET` | `/api/subscriptions?email={email}` | Get all active subscriptions for a given email |

## 6. Unit testing

#### **To run use:** 
```bash
go test ./...
```

Unit tests cover most of the service and handler layers. Which is exactly the core business logic of the API. 


