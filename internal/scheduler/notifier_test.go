package scheduler_test

import (
	"errors"
	"testing"

	"github-release-notifier/internal/domain"
	"github-release-notifier/internal/scheduler"

	"github.com/stretchr/testify/assert"
)

// --- mocks ---

type mockStore struct {
	subs          []domain.Subscription
	findErr       error
	updateErr     error
	updatedCalls  int
	lastUpdatedID  int
	lastUpdatedTag string
}

func (m *mockStore) FindAllConfirmed() ([]domain.Subscription, error) {
	return m.subs, m.findErr
}

func (m *mockStore) UpdateLastSeenTag(id int, tag string) error {
	m.updatedCalls++
	m.lastUpdatedID = id
	m.lastUpdatedTag = tag
	return m.updateErr
}

type mockChecker struct {
	releases  map[string]string
	err       error
	callCount map[string]int
}

func newMockChecker() *mockChecker {
	return &mockChecker{
		releases:  make(map[string]string),
		callCount: make(map[string]int),
	}
}

func (m *mockChecker) GetLatestRelease(repo string) (*domain.Release, error) {
	m.callCount[repo]++
	if m.err != nil {
		return nil, m.err
	}
	return &domain.Release{TagName: m.releases[repo]}, nil
}

type mockSender struct {
	sentTo  []string
	sentTag []string
	err     error
}

func (m *mockSender) SendReleaseNotification(to, _, tag string) error {
	if m.err != nil {
		return m.err
	}
	m.sentTo = append(m.sentTo, to)
	m.sentTag = append(m.sentTag, tag)
	return nil
}

// --- tests ---

func TestNotifier_NoSubscriptions(t *testing.T) {
	store := &mockStore{}
	n := scheduler.NewNotifier(store, newMockChecker(), &mockSender{})
	n.Run()
	assert.Empty(t, store.updatedCalls)
}

func TestNotifier_NewRelease(t *testing.T) {
	store := &mockStore{
		subs: []domain.Subscription{
			{ID: 1, Email: "a@b.com", Repo: "owner/repo", LastSeenTag: "v1.0.0"},
		},
	}
	checker := newMockChecker()
	checker.releases["owner/repo"] = "v2.0.0"
	sender := &mockSender{}

	scheduler.NewNotifier(store, checker, sender).Run()

	assert.Equal(t, []string{"a@b.com"}, sender.sentTo)
	assert.Equal(t, []string{"v2.0.0"}, sender.sentTag)
	assert.Equal(t, 1, store.updatedCalls)
	assert.Equal(t, "v2.0.0", store.lastUpdatedTag)
}

func TestNotifier_SameTag_NoNotification(t *testing.T) {
	store := &mockStore{
		subs: []domain.Subscription{
			{ID: 1, Email: "a@b.com", Repo: "owner/repo", LastSeenTag: "v1.0.0"},
		},
	}
	checker := newMockChecker()
	checker.releases["owner/repo"] = "v1.0.0"
	sender := &mockSender{}

	scheduler.NewNotifier(store, checker, sender).Run()

	assert.Empty(t, sender.sentTo)
	assert.Equal(t, 0, store.updatedCalls)
}

func TestNotifier_EmptyTag_NoNotification(t *testing.T) {
	store := &mockStore{
		subs: []domain.Subscription{
			{ID: 1, Email: "a@b.com", Repo: "owner/repo", LastSeenTag: ""},
		},
	}
	checker := newMockChecker()
	checker.releases["owner/repo"] = "" // no release published yet

	sender := &mockSender{}

	scheduler.NewNotifier(store, checker, sender).Run()

	assert.Empty(t, sender.sentTo)
	assert.Equal(t, 0, store.updatedCalls)
}

func TestNotifier_GitHubError_SkipsSubscriber(t *testing.T) {
	store := &mockStore{
		subs: []domain.Subscription{
			{ID: 1, Email: "a@b.com", Repo: "owner/repo", LastSeenTag: "v1.0.0"},
		},
	}
	checker := newMockChecker()
	checker.err = errors.New("rate limited")
	sender := &mockSender{}

	// must not panic
	scheduler.NewNotifier(store, checker, sender).Run()

	assert.Empty(t, sender.sentTo)
	assert.Equal(t, 0, store.updatedCalls)
}

func TestNotifier_MailerError_DoesNotUpdateTag(t *testing.T) {
	store := &mockStore{
		subs: []domain.Subscription{
			{ID: 1, Email: "a@b.com", Repo: "owner/repo", LastSeenTag: "v1.0.0"},
		},
	}
	checker := newMockChecker()
	checker.releases["owner/repo"] = "v2.0.0"
	sender := &mockSender{err: errors.New("smtp failure")}

	scheduler.NewNotifier(store, checker, sender).Run()

	assert.Equal(t, 0, store.updatedCalls, "UpdateLastSeenTag must not be called after mailer error")
}

func TestNotifier_DeduplicatesRepoLookup(t *testing.T) {
	store := &mockStore{
		subs: []domain.Subscription{
			{ID: 1, Email: "a@b.com", Repo: "owner/repo", LastSeenTag: "v1.0.0"},
			{ID: 2, Email: "c@d.com", Repo: "owner/repo", LastSeenTag: "v1.0.0"},
		},
	}
	checker := newMockChecker()
	checker.releases["owner/repo"] = "v2.0.0"
	sender := &mockSender{}

	scheduler.NewNotifier(store, checker, sender).Run()

	assert.Equal(t, 1, checker.callCount["owner/repo"], "GitHub API must be called only once per repo")
	assert.Equal(t, 2, len(sender.sentTo), "both subscribers must be notified")
	assert.Equal(t, 2, store.updatedCalls)
}

func TestNotifier_FindAllConfirmedError(t *testing.T) {
	store := &mockStore{findErr: errors.New("db down")}
	sender := &mockSender{}

	// must not panic
	scheduler.NewNotifier(store, newMockChecker(), sender).Run()

	assert.Empty(t, sender.sentTo)
}
