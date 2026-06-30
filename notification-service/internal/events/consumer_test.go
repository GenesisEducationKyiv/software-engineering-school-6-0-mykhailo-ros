package events

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

type mockMailer struct {
	calls []struct{ to, repo, url string }
	err   error
}

func (m *mockMailer) SendConfirmation(to, repo, url string) error {
	m.calls = append(m.calls, struct{ to, repo, url string }{to, repo, url})
	return m.err
}

func TestHandle_Success(t *testing.T) {
	m := &mockMailer{}
	c := &Consumer{mailer: m}

	body, _ := json.Marshal(map[string]string{
		"email": "test@example.com", "repo": "owner/repo", "confirm_url": "http://example.com/confirm/abc",
	})
	err := c.handle(body)
	assert.NoError(t, err)
	assert.Len(t, m.calls, 1)
	assert.Equal(t, "test@example.com", m.calls[0].to)
	assert.Equal(t, "owner/repo", m.calls[0].repo)
	assert.Equal(t, "http://example.com/confirm/abc", m.calls[0].url)
}

func TestHandle_MailerError(t *testing.T) {
	m := &mockMailer{err: errors.New("smtp failure")}
	c := &Consumer{mailer: m}

	body, _ := json.Marshal(map[string]string{
		"email": "test@example.com", "repo": "owner/repo", "confirm_url": "http://example.com/confirm/abc",
	})
	err := c.handle(body)
	assert.EqualError(t, err, "smtp failure")
}

func TestHandle_InvalidJSON(t *testing.T) {
	m := &mockMailer{}
	c := &Consumer{mailer: m}
	err := c.handle([]byte("not-json"))
	assert.Error(t, err)
	assert.Len(t, m.calls, 0)
}
