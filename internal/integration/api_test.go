//go:build integration

package integration_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helper: subscribe and return the response
func subscribe(t *testing.T, email, repo string) *http.Response {
	t.Helper()
	resp, err := doJSON(http.MethodPost, apiURL("/api/subscribe"),
		map[string]string{"email": email, "repo": repo})
	require.NoError(t, err)
	return resp
}

// helper: read confirm_token from DB directly for a given email+repo
func confirmTokenFor(t *testing.T, email, repo string) string {
	t.Helper()
	var token string
	err := testDB.QueryRow(
		`SELECT confirm_token FROM subscriptions WHERE email=$1 AND repo=$2`, email, repo,
	).Scan(&token)
	require.NoError(t, err)
	return token
}

// helper: read unsubscribe_token from DB directly for a given email+repo
func unsubscribeTokenFor(t *testing.T, email, repo string) string {
	t.Helper()
	var token string
	err := testDB.QueryRow(
		`SELECT unsubscribe_token FROM subscriptions WHERE email=$1 AND repo=$2`, email, repo,
	).Scan(&token)
	require.NoError(t, err)
	return token
}

// --- Subscribe ---

func TestSubscribe_Success(t *testing.T) {
	truncateDB(t)

	resp := subscribe(t, "user@example.com", "owner/repo")
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// confirmation email must have been sent
	assert.Equal(t, 1, smtpStub.MessageCount())
}

func TestSubscribe_AlreadySubscribed(t *testing.T) {
	truncateDB(t)

	subscribe(t, "user@example.com", "owner/repo")
	resp := subscribe(t, "user@example.com", "owner/repo")
	assert.Equal(t, http.StatusConflict, resp.StatusCode)
}

func TestSubscribe_RepoNotFound(t *testing.T) {
	truncateDB(t)
	setGithubRepo(http.StatusNotFound)

	resp := subscribe(t, "user@example.com", "ghost/missing")
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestSubscribe_RateLimited(t *testing.T) {
	truncateDB(t)
	setGithubRepo(http.StatusTooManyRequests)

	resp := subscribe(t, "user@example.com", "owner/repo")
	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
}

func TestSubscribe_InvalidEmail(t *testing.T) {
	truncateDB(t)

	resp := subscribe(t, "notanemail", "owner/repo")
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestSubscribe_InvalidRepo(t *testing.T) {
	truncateDB(t)

	resp := subscribe(t, "user@example.com", "noslash")
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// --- Confirm ---

func TestConfirm_Success(t *testing.T) {
	truncateDB(t)
	subscribe(t, "user@example.com", "owner/repo")

	token := confirmTokenFor(t, "user@example.com", "owner/repo")
	resp, err := http.Get(apiURL("/api/confirm/" + token))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// subscription must now be confirmed
	var confirmed bool
	err = testDB.QueryRow(
		`SELECT confirmed FROM subscriptions WHERE confirm_token=$1`, token,
	).Scan(&confirmed)
	require.NoError(t, err)
	assert.True(t, confirmed)
}

func TestConfirm_NotFound(t *testing.T) {
	truncateDB(t)

	token := strings.Repeat("a", 32)
	resp, err := http.Get(apiURL("/api/confirm/" + token))
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestConfirm_InvalidToken(t *testing.T) {
	truncateDB(t)

	resp, err := http.Get(apiURL("/api/confirm/tooshort"))
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// --- Unsubscribe ---

func TestUnsubscribe_Success(t *testing.T) {
	truncateDB(t)
	subscribe(t, "user@example.com", "owner/repo")

	token := unsubscribeTokenFor(t, "user@example.com", "owner/repo")
	resp, err := http.Get(apiURL("/api/unsubscribe/" + token))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// row must be gone
	var count int
	testDB.QueryRow(`SELECT COUNT(*) FROM subscriptions WHERE unsubscribe_token=$1`, token).Scan(&count) //nolint:errcheck
	assert.Equal(t, 0, count)
}

func TestUnsubscribe_NotFound(t *testing.T) {
	truncateDB(t)

	token := strings.Repeat("b", 32)
	resp, err := http.Get(apiURL("/api/unsubscribe/" + token))
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestUnsubscribe_InvalidToken(t *testing.T) {
	truncateDB(t)

	resp, err := http.Get(apiURL("/api/unsubscribe/short"))
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// --- GetSubscriptions ---

func TestGetSubscriptions_Empty(t *testing.T) {
	truncateDB(t)

	resp, err := http.Get(apiURL("/api/subscriptions?email=nobody@example.com"))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result []any
	decodeBody(t, resp, &result)
	assert.Empty(t, result)
}

func TestGetSubscriptions_OnlyConfirmedReturned(t *testing.T) {
	truncateDB(t)
	// subscribe but do NOT confirm
	subscribe(t, "user@example.com", "owner/repo")

	resp, err := http.Get(apiURL("/api/subscriptions?email=user@example.com"))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result []any
	decodeBody(t, resp, &result)
	assert.Empty(t, result, "unconfirmed subscription must not appear")
}

func TestGetSubscriptions_InvalidEmail(t *testing.T) {
	truncateDB(t)

	resp, err := http.Get(apiURL("/api/subscriptions?email=bad"))
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// --- Full lifecycle ---

func TestFullLifecycle(t *testing.T) {
	truncateDB(t)

	const email = "lifecycle@example.com"
	const repo = "owner/repo"

	// 1. subscribe
	resp := subscribe(t, email, repo)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, 1, smtpStub.MessageCount())

	// 2. not in subscriptions yet (unconfirmed)
	resp2, _ := http.Get(apiURL("/api/subscriptions?email=" + email))
	var subs []map[string]any
	decodeBody(t, resp2, &subs)
	assert.Empty(t, subs)

	// 3. confirm
	confirmToken := confirmTokenFor(t, email, repo)
	resp3, _ := http.Get(apiURL("/api/confirm/" + confirmToken))
	require.Equal(t, http.StatusOK, resp3.StatusCode)

	// 4. now appears in subscriptions
	resp4, _ := http.Get(apiURL("/api/subscriptions?email=" + email))
	json.NewDecoder(resp4.Body).Decode(&subs) //nolint:errcheck
	resp4.Body.Close()
	require.Len(t, subs, 1)
	assert.Equal(t, repo, subs[0]["repo"])
	assert.Equal(t, true, subs[0]["confirmed"])

	// 5. unsubscribe
	unsubToken := unsubscribeTokenFor(t, email, repo)
	resp5, _ := http.Get(apiURL("/api/unsubscribe/" + unsubToken))
	require.Equal(t, http.StatusOK, resp5.StatusCode)

	// 6. gone from subscriptions
	resp6, _ := http.Get(apiURL("/api/subscriptions?email=" + email))
	json.NewDecoder(resp6.Body).Decode(&subs) //nolint:errcheck
	resp6.Body.Close()
	assert.Empty(t, subs)
}
