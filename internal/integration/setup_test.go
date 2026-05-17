//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github-release-notifier/internal/cache"
	"github-release-notifier/internal/db"
	"github-release-notifier/internal/github"
	"github-release-notifier/internal/handler"
	"github-release-notifier/internal/mailer"
	"github-release-notifier/internal/repository"
	"github-release-notifier/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/golang-migrate/migrate/v4"
	pgmigrate "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// shared state set up once in TestMain
var (
	testServer   *httptest.Server
	testDB       *sql.DB
	smtpStub     *fakeSMTPServer

	githubStubMu     sync.Mutex
	githubRepoStatus = http.StatusOK
	githubReleaseTag = "v1.0.0"
)

func setGithubRepo(status int) {
	githubStubMu.Lock()
	githubRepoStatus = status
	githubStubMu.Unlock()
}

func setGithubRelease(tag string) {
	githubStubMu.Lock()
	githubReleaseTag = tag
	githubStubMu.Unlock()
}

func resetGithubStub() {
	githubStubMu.Lock()
	githubRepoStatus = http.StatusOK
	githubReleaseTag = "v1.0.0"
	githubStubMu.Unlock()
}

func TestMain(m *testing.M) {
	ctx := context.Background()
	gin.SetMode(gin.TestMode)

	// -- postgres --
	pgC, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("testnotifier"),
		tcpostgres.WithUsername("postgres"),
		tcpostgres.WithPassword("postgres"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(60*time.Second),
		),
	)
	mustOK(err, "start postgres")
	defer pgC.Terminate(ctx) //nolint:errcheck

	pgHost, _ := pgC.Host(ctx)
	pgPort, _ := pgC.MappedPort(ctx, "5432/tcp")

	testDB, err = db.Connect(pgHost, pgPort.Port(), "postgres", "postgres", "testnotifier")
	mustOK(err, "db.Connect")
	defer testDB.Close() //nolint:errcheck

	mustOK(runMigrations(testDB), "run migrations")

	// -- redis --
	redisC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "redis:7-alpine",
			ExposedPorts: []string{"6379/tcp"},
			WaitingFor:   wait.ForListeningPort("6379/tcp").WithStartupTimeout(30 * time.Second),
		},
		Started: true,
	})
	mustOK(err, "start redis")
	defer redisC.Terminate(ctx) //nolint:errcheck

	redisHost, _ := redisC.Host(ctx)
	redisPort, _ := redisC.MappedPort(ctx, "6379/tcp")
	redisAddr := fmt.Sprintf("%s:%s", redisHost, redisPort.Port())

	// -- github stub --
	githubSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		githubStubMu.Lock()
		repoStatus := githubRepoStatus
		releaseTag := githubReleaseTag
		githubStubMu.Unlock()

		if strings.HasSuffix(r.URL.Path, "/releases/latest") {
			if releaseTag == "" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"tag_name": releaseTag}) //nolint:errcheck
			return
		}
		w.WriteHeader(repoStatus)
	}))
	defer githubSrv.Close()

	// -- smtp stub --
	smtpStub, err = startFakeSMTP()
	mustOK(err, "start smtp stub")
	defer smtpStub.Close()

	// -- wire application --
	cacheClient := cache.NewCacheWithAddr(redisAddr)
	githubClient := github.NewTestClient("", githubSrv.URL, cacheClient)
	mailerClient := mailer.NewMailer(smtpStub.Addr(), smtpStub.Port(), "testuser", "testpass", "test@test.com")
	repo := repository.NewSubscriptionRepo(testDB)
	svc := service.NewSubscription(repo, githubClient, mailerClient, "http://localhost")
	h := handler.NewSubscriptionHandler(svc)

	r := gin.New()
	r.POST("/api/subscribe", h.Subscribe)
	r.GET("/api/confirm/:token", h.Confirm)
	r.GET("/api/unsubscribe/:token", h.Unsubscribe)
	r.GET("/api/subscriptions", h.GetSubscriptions)
	testServer = httptest.NewServer(r)
	defer testServer.Close()

	os.Exit(m.Run())
}

// truncateDB resets all rows and resets helper stubs.
func truncateDB(t *testing.T) {
	t.Helper()
	_, err := testDB.Exec("TRUNCATE TABLE subscriptions RESTART IDENTITY")
	require.NoError(t, err)
	smtpStub.Reset()
	resetGithubStub()
}

// apiURL builds a full URL for a path against the test server.
func apiURL(path string) string { return testServer.URL + path }

// doJSON sends a JSON request and returns the response.
func doJSON(method, url string, body any) (*http.Response, error) {
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, url, r)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return http.DefaultClient.Do(req)
}

// decodeBody reads JSON from an http.Response body into dst.
func decodeBody(t *testing.T, resp *http.Response, dst any) {
	t.Helper()
	defer resp.Body.Close()
	require.NoError(t, json.NewDecoder(resp.Body).Decode(dst))
}

// runMigrations applies migrations from the project root, computed via the
// source file path so it works regardless of working directory.
func runMigrations(sqlDB *sql.DB) error {
	_, thisFile, _, _ := runtime.Caller(0)
	// thisFile: …/internal/integration/setup_test.go — project root is 2 dirs up
	projectRoot := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "../.."))
	migrationsPath := "file://" + filepath.Join(projectRoot, "migrations")

	driver, err := pgmigrate.WithInstance(sqlDB, &pgmigrate.Config{})
	if err != nil {
		return err
	}
	mg, err := migrate.NewWithDatabaseInstance(migrationsPath, "postgres", driver)
	if err != nil {
		return err
	}
	if err := mg.Up(); err != nil && err != migrate.ErrNoChange {
		return err
	}
	return nil
}

func mustOK(err error, msg string) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "integration: %s: %v\n", msg, err)
		os.Exit(1)
	}
}
