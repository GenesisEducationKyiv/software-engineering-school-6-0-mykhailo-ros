package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github-release-notifier/internal/domain"

	"github.com/gin-gonic/gin"
)

type mockService struct {
	subscribeErr   error
	confirmErr     error
	unsubscribeErr error
	subscriptions  []domain.Subscription
}

func (m *mockService) Subscribe(email, repo string) error {
	return m.subscribeErr
}

func (m *mockService) Confirm(token string) error {
	return m.confirmErr
}

func (m *mockService) Unsubscribe(token string) error {
	return m.unsubscribeErr
}

func (m *mockService) GetSubscriptions(email string) ([]domain.Subscription, error) {
	return m.subscriptions, nil
}

func setupRouter(svc SubscriptionService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.Default()
	h := NewSubscriptionHandler(svc)
	r.POST("/api/subscribe", h.Subscribe)
	r.GET("/api/confirm/:token", h.Confirm)
	r.GET("/api/unsubscribe/:token", h.Unsubscribe)
	r.GET("/api/subscriptions", h.GetSubscriptions)
	return r
}

func TestSubscribeHandler_Success(t *testing.T) {
	r := setupRouter(&mockService{})

	form := url.Values{}
	form.Set("email", "test@test.com")
	form.Set("repo", "golang/go")

	req := httptest.NewRequest("POST", "/api/subscribe", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestSubscribeHandler_InvalidEmail(t *testing.T) {
	r := setupRouter(&mockService{})

	form := url.Values{}
	form.Set("email", "notanemail")
	form.Set("repo", "golang/go")

	req := httptest.NewRequest("POST", "/api/subscribe", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestSubscribeHandler_InvalidRepo(t *testing.T) {
	r := setupRouter(&mockService{})

	form := url.Values{}
	form.Set("email", "test@test.com")
	form.Set("repo", "invalidrepo")

	req := httptest.NewRequest("POST", "/api/subscribe", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestSubscribeHandler_RepoNotFound(t *testing.T) {
	r := setupRouter(&mockService{subscribeErr: fmt.Errorf("repo not found")})

	form := url.Values{}
	form.Set("email", "test@test.com")
	form.Set("repo", "golang/go")

	req := httptest.NewRequest("POST", "/api/subscribe", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestConfirmHandler_Success(t *testing.T) {
	r := setupRouter(&mockService{})
	token := strings.Repeat("a", 32)

	req := httptest.NewRequest("GET", "/api/confirm/"+token, nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestConfirmHandler_InvalidToken(t *testing.T) {
	r := setupRouter(&mockService{})

	req := httptest.NewRequest("GET", "/api/confirm/short", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestConfirmHandler_TokenNotFound(t *testing.T) {
	r := setupRouter(&mockService{confirmErr: fmt.Errorf("token not found")})
	token := strings.Repeat("a", 32)

	req := httptest.NewRequest("GET", "/api/confirm/"+token, nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestGetSubscriptionsHandler_Success(t *testing.T) {
	subs := []domain.Subscription{{Email: "test@test.com", Repo: "golang/go", Confirmed: true}}
	r := setupRouter(&mockService{subscriptions: subs})

	req := httptest.NewRequest("GET", "/api/subscriptions?email=test@test.com", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var result []map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&result)
	if err != nil {
		t.Errorf("error while decoding")
	}
	if len(result) != 1 {
		t.Errorf("expected 1 subscription, got %d", len(result))
	}
}

func TestGetSubscriptionsHandler_InvalidEmail(t *testing.T) {
	r := setupRouter(&mockService{})

	req := httptest.NewRequest("GET", "/api/subscriptions?email=notanemail", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}
