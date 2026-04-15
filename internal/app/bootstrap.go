package app

import (
	"context"
	"log"
	"os"
	"time"

	"command-task/internal/cache"
	"command-task/internal/config"
	"command-task/internal/provider/clickup"
	syncengine "command-task/internal/sync"
	"command-task/internal/tui"
)

func Bootstrap(ctx context.Context) (*Runtime, error) {
	cfg := config.Load()

	db, err := cache.Open(cfg.DBPath)
	if err != nil {
		return nil, err
	}

	repo := cache.NewRepository(db)

	var (
		engine     *syncengine.Engine
		statusLine string
	)

	token, tokenErr := clickup.TokenFromEnv()
	if tokenErr != nil {
		statusLine = "ClickUp token not configured (set CLICKUP_API_TOKEN)"
	} else {
		provider := clickup.NewFromToken(token)
		engine = syncengine.NewEngine(repo, provider, log.New(os.Stderr, "sync: ", log.LstdFlags), 20*time.Second)
		statusLine = "ClickUp sync running in background"
	}

	model := tui.NewRootModel(repo, engine, statusLine)
	return NewRuntime(db, repo, engine, model), nil
}
