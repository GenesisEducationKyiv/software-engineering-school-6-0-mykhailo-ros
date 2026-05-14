package scheduler

import (
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

type Scheduler struct {
	repo     NotificationStore
	github   ReleaseChecker
	mailer   NotificationSender
	interval time.Duration
}

func NewScheduler(repo NotificationStore, github ReleaseChecker, mailer NotificationSender, interval time.Duration) *Scheduler {
	return &Scheduler{repo: repo, github: github, mailer: mailer, interval: interval}
}

func (s *Scheduler) checkAndNotify() {
	subs, err := s.repo.FindAllConfirmed()
	if err != nil {
		log.Printf("scheduler: failed to fetch subscription: %v", err)
		return
	}

	seen := make(map[string]string)

	for _, sub := range subs {
		tag, ok := seen[sub.Repo]
		if !ok {
			release, err := s.github.GetLatestRelease(sub.Repo)
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

		if err := s.mailer.SendReleaseNotification(sub.Email, sub.Repo, tag); err != nil {
			log.Printf("scheduler: failed to send email to %s: %v", sub.Email, err)
			continue
		}

		if err := s.repo.UpdateLastSeenTag(sub.ID, tag); err != nil {
			log.Printf("scheduler: failed to update last_seen_tag for %s: %v", sub.Email, err)
		}
	}
}

func (s *Scheduler) Start() {
	go func() {
		for {
			s.checkAndNotify()
			time.Sleep(s.interval)
		}
	}()
}
