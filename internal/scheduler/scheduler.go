package scheduler

import (
	"github-release-notifier/internal/github"
	"github-release-notifier/internal/mailer"
	"github-release-notifier/internal/repository"
	"log"
	"time"
)

type Scheduler struct {
	repo     *repository.SubscriptionRepo
	github   *github.Client
	mailer   *mailer.Mailer
	interval time.Duration
}

func NewScheduler(repo *repository.SubscriptionRepo, github *github.Client, mailer *mailer.Mailer, interval time.Duration) *Scheduler {
	return &Scheduler{repo: repo, github: github, mailer: mailer, interval: interval}
}

func (s *Scheduler) check() {
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
			if release == nil {
				seen[sub.Repo] = ""
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
			s.check()
			time.Sleep(s.interval)
		}
	}()
}
