package app

import (
	"context"
	"log"
	"os"
	"time"

	"lazy-click/internal/cache"
	"lazy-click/internal/config"
	"lazy-click/internal/provider/clickup"
	syncengine "lazy-click/internal/sync"
	"lazy-click/internal/tui"
)

func Bootstrap(ctx context.Context) (*Runtime, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	db, err := cache.Open(cfg.DBPath)
	if err != nil {
		return nil, err
	}

	repo := cache.NewRepository(db)

	var (
		engine     *syncengine.Engine
		statusLine string
		provider   = "ClickUp"
	)

	token, tokenErr := clickup.TokenFromEnv()
	if tokenErr != nil {
		statusLine = "ClickUp token not configured (set CLICKUP_API_TOKEN)"
	} else {
		clickupProvider := clickup.NewFromToken(token)
		engine = syncengine.NewEngine(repo, clickupProvider, log.New(os.Stderr, "sync: ", log.LstdFlags), 15*time.Minute)
		statusLine = ""
	}

	model := tui.NewRootModel(repo, engine, provider, statusLine)
	return NewRuntime(db, repo, engine, model), nil
}
