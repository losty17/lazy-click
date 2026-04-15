package config

import (
	"errors"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DBPath string
}

func Load() (Config, error) {
	if err := loadDotEnv(); err != nil {
		return Config{}, err
	}

	dbPath := os.Getenv("COMMAND_TASK_DB_PATH")
	if dbPath == "" {
		dbPath = "lazy-click.db"
	}

	return Config{
		DBPath: dbPath,
	}, nil
}

func loadDotEnv() error {
	err := godotenv.Load(".env")
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
