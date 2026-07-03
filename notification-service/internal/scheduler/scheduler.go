package scheduler

import (
	"context"
	"log/slog"
	"notification-service/internal/domain"
	"time"
)

type NotificationStore interface {
	FindAllConfirmed(ctx context.Context) ([]domain.Subscription, error)
	UpdateLastSeenTag(ctx context.Context, id int, tag string) error
}

type ReleaseChecker interface {
	GetLatestRelease(repo string) (*domain.Release, error)
}

type NotificationSender interface {
	SendReleaseNotification(to, repo, tag string) error
}

type NotificationJob interface {
	Run(ctx context.Context)
}

type Notifier struct {
	repo   NotificationStore
	github ReleaseChecker
	mailer NotificationSender
}

func NewNotifier(repo NotificationStore, github ReleaseChecker, mailer NotificationSender) *Notifier {
	return &Notifier{repo: repo, github: github, mailer: mailer}
}

func (n *Notifier) Run(ctx context.Context) {
	subs, err := n.repo.FindAllConfirmed(ctx)
	if err != nil {
		slog.Error("scheduler: failed to fetch subscriptions", "error", err)
		return
	}

	seen := make(map[string]string)

	for _, sub := range subs {
		tag, ok := seen[sub.Repo]
		if !ok {
			release, err := n.github.GetLatestRelease(sub.Repo)
			if err != nil {
				slog.Error("scheduler: failed to get release", "repo", sub.Repo, "error", err)
				seen[sub.Repo] = ""
				continue
			}
			tag = release.TagName
			seen[sub.Repo] = tag
		}

		if tag == "" || tag == sub.LastSeenTag {
			continue
		}

		if err := n.mailer.SendReleaseNotification(sub.Email, sub.Repo, tag); err != nil {
			slog.Error("scheduler: failed to send email", "email", sub.Email, "error", err)
			continue
		}

		if err := n.repo.UpdateLastSeenTag(ctx, sub.ID, tag); err != nil {
			slog.Error("scheduler: failed to update last_seen_tag", "email", sub.Email, "error", err)
		}
	}
}

type Scheduler struct {
	job      NotificationJob
	interval time.Duration
	done     chan struct{}
}

func NewScheduler(job NotificationJob, interval time.Duration) *Scheduler {
	return &Scheduler{job: job, interval: interval, done: make(chan struct{})}
}

func (s *Scheduler) Start(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	go func() {
		defer close(s.done)
		defer ticker.Stop()
		safeRun := func() {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("scheduler: panic in job", "error", r)
				}
			}()
			s.job.Run(ctx)
		}
		safeRun()
		for {
			select {
			case <-ticker.C:
				safeRun()
			case <-ctx.Done():
				return
			}
		}
	}()
}

// Wait blocks until the scheduler's background goroutine has exited, so
// callers can join it before tearing down resources it depends on.
func (s *Scheduler) Wait() {
	<-s.done
}
