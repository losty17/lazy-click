package config

import "os"

type Config struct {
	DBPath string
}

func Load() Config {
	dbPath := os.Getenv("COMMAND_TASK_DB_PATH")
	if dbPath == "" {
		dbPath = "command-task.db"
	}

	return Config{
		DBPath: dbPath,
	}
}
