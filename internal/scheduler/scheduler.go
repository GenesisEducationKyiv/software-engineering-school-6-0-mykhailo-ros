package scheduler

import (
	"context"
	"github-release-notifier/internal/domain"
	"github-release-notifier/internal/repository"
	"log"
	"time"
)

type NotificationStore interface {
	FindAllConfirmed() ([]repository.Subscription, error)
	UpdateLastSeenTag(id int, tag string) error
}

type ReleaseChecker interface {
	GetLatestRelease(repo string) (*domain.Release, error)
}

type NotificationSender interface {
	SendReleaseNotification(to, repo, tag string) error
}

type NotificationJob interface {
	Run()
}

type Notifier struct {
	repo   NotificationStore
	github ReleaseChecker
	mailer NotificationSender
}

func NewNotifier(repo NotificationStore, github ReleaseChecker, mailer NotificationSender) *Notifier {
	return &Notifier{repo: repo, github: github, mailer: mailer}
}

func (n *Notifier) Run() {
	subs, err := n.repo.FindAllConfirmed()
	if err != nil {
		log.Printf("scheduler: failed to fetch subscriptions: %v", err)
		return
	}

	seen := make(map[string]string)

	for _, sub := range subs {
		tag, ok := seen[sub.Repo]
		if !ok {
			release, err := n.github.GetLatestRelease(sub.Repo)
			if err != nil {
				log.Printf("scheduler: failed to get release for %s: %v", sub.Repo, err)
				continue
			}
			tag = release.TagName
			seen[sub.Repo] = tag
		}

		if tag == "" || tag == sub.LastSeenTag {
			continue
		}

		if err := n.mailer.SendReleaseNotification(sub.Email, sub.Repo, tag); err != nil {
			log.Printf("scheduler: failed to send email to %s: %v", sub.Email, err)
			continue
		}

		if err := n.repo.UpdateLastSeenTag(sub.ID, tag); err != nil {
			log.Printf("scheduler: failed to update last_seen_tag for %s: %v", sub.Email, err)
		}
	}
}

type Scheduler struct {
	job      NotificationJob
	interval time.Duration
}

func NewScheduler(job NotificationJob, interval time.Duration) *Scheduler {
	return &Scheduler{job: job, interval: interval}
}

func (s *Scheduler) Start(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	go func() {
		defer ticker.Stop()
		s.job.Run()
		for {
			select {
			case <-ticker.C:
				s.job.Run()
			case <-ctx.Done():
				return
			}
		}
	}()
}
