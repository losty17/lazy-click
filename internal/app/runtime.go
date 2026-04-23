package app

import (
	"context"

	"lazy-click/internal/cache"
	syncengine "lazy-click/internal/sync"
	"lazy-click/internal/tui"

	tea "github.com/charmbracelet/bubbletea"
	"gorm.io/gorm"
)

type Runtime struct {
	db      *gorm.DB
	repo    *cache.Repository
	syncers []*syncengine.Engine
	model   tui.RootModel
}

func NewRuntime(db *gorm.DB, repo *cache.Repository, syncers []*syncengine.Engine, model tui.RootModel) *Runtime {
	return &Runtime{
		db:      db,
		repo:    repo,
		syncers: syncers,
		model:   model,
	}
}

func (r *Runtime) Run(ctx context.Context) error {
	for _, syncer := range r.syncers {
		if syncer == nil {
			continue
		}
		go func(engine *syncengine.Engine) {
			_ = engine.Run(ctx)
		}(syncer)
	}
	p := tea.NewProgram(r.model, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
