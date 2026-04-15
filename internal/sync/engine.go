package syncengine

import (
	"context"
	"log"
	"time"

	"lazy-click/internal/cache"
	"lazy-click/internal/provider"
)

type Engine struct {
	repo     *cache.Repository
	provider provider.ProjectProvider
	logger   *log.Logger
	interval time.Duration
}

func NewEngine(repo *cache.Repository, provider provider.ProjectProvider, logger *log.Logger, interval time.Duration) *Engine {
	if interval <= 0 {
		interval = 10 * time.Second
	}
	return &Engine{
		repo:     repo,
		provider: provider,
		logger:   logger,
		interval: interval,
	}
}

func (e *Engine) Run(ctx context.Context) error {
	ticker := time.NewTicker(e.interval)
	defer ticker.Stop()

	for {
		if err := e.Cycle(ctx); err != nil && e.logger != nil {
			e.logger.Printf("sync cycle failed: %v", err)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (e *Engine) Cycle(ctx context.Context) error {
	if err := e.PullOnce(ctx); err != nil {
		return err
	}
	return e.PushOnce(ctx)
}
