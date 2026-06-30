package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	DBHost        string
	DBPort        string
	DBUser        string
	DBPassword    string
	DBName        string
	BaseURL       string
	GithubToken   string
	InternalToken string
	RabbitMQURL   string
}

func Load() (*Config, error) {
	cfg := &Config{
		DBHost:        os.Getenv("DB_HOST"),
		DBPort:        os.Getenv("DB_PORT"),
		DBUser:        os.Getenv("DB_USER"),
		DBPassword:    os.Getenv("DB_PASSWORD"),
		DBName:        os.Getenv("DB_NAME"),
		BaseURL:       os.Getenv("BASE_URL"),
		GithubToken:   os.Getenv("GITHUB_TOKEN"),
		InternalToken: os.Getenv("INTERNAL_TOKEN"),
		RabbitMQURL:   os.Getenv("RABBITMQ_URL"),
	}

	required := map[string]string{
		"DB_HOST":      cfg.DBHost,
		"DB_PORT":      cfg.DBPort,
		"DB_USER":      cfg.DBUser,
		"DB_PASSWORD":  cfg.DBPassword,
		"DB_NAME":      cfg.DBName,
		"BASE_URL":     cfg.BaseURL,
		"RABBITMQ_URL": cfg.RabbitMQURL,
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
