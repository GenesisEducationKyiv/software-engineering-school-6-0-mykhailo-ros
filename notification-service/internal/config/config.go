package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	SubscriptionServiceURL string
	SMTPHost               string
	SMTPPort               string
	SMTPUsername           string
	SMTPPassword           string
	SMTPFrom               string
	GithubToken            string
	RedisAddr              string
}

func Load() (*Config, error) {
	cfg := &Config{
		SubscriptionServiceURL: os.Getenv("SUBSCRIPTION_SERVICE_URL"),
		SMTPHost:               os.Getenv("SMTP_HOST"),
		SMTPPort:               os.Getenv("SMTP_PORT"),
		SMTPUsername:           os.Getenv("SMTP_USERNAME"),
		SMTPPassword:           os.Getenv("SMTP_PASSWORD"),
		SMTPFrom:               os.Getenv("SMTP_FROM"),
		GithubToken:            os.Getenv("GITHUB_TOKEN"),
		RedisAddr:              os.Getenv("REDIS_ADDR"),
	}

	required := map[string]string{
		"SUBSCRIPTION_SERVICE_URL": cfg.SubscriptionServiceURL,
		"SMTP_HOST":                cfg.SMTPHost,
		"SMTP_PORT":                cfg.SMTPPort,
		"SMTP_FROM":                cfg.SMTPFrom,
		"REDIS_ADDR":               cfg.RedisAddr,
	}

	var missing []string
	for key, val := range required {
		if val == "" {
			missing = append(missing, key)
		}
	}

	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}

	return cfg, nil
}
