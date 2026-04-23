package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
)

var buildDefaultClickUpClientID string
var buildDefaultOAuthBackendURL string

type Config struct {
	DBPath              string
	DefaultProvider     string
	ClickUpAPIToken     string
	ClickUpClientID     string
	ClickUpClientSecret string
	OAuthBackendURL     string
}

func Load() (Config, error) {
	if err := loadDotEnv(); err != nil {
		return Config{}, err
	}

	dbPath := os.Getenv("COMMAND_TASK_DB_PATH")
	if dbPath == "" {
		dbPath = defaultDBPath()
	}
	defaultProvider := os.Getenv("LAZY_CLICK_DEFAULT_PROVIDER")

	return Config{
		DBPath:              dbPath,
		DefaultProvider:     strings.TrimSpace(defaultProvider),
		ClickUpAPIToken:     os.Getenv("CLICKUP_API_TOKEN"),
		ClickUpClientID:     firstNonEmpty(os.Getenv("CLICKUP_CLIENT_ID"), buildDefaultClickUpClientID),
		ClickUpClientSecret: os.Getenv("CLICKUP_CLIENT_SECRET"),
		OAuthBackendURL:     firstNonEmpty(os.Getenv("LAZY_CLICK_OAUTH_BACKEND_URL"), buildDefaultOAuthBackendURL),
	}, nil
}

func defaultDBPath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "lazy-click.db"
	}
	return filepath.Join(home, ".local", "share", "lazy-click", "lazy-click.db")
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func loadDotEnv() error {
	err := godotenv.Load(".env")
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
