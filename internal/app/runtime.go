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
	db     *gorm.DB
	repo   *cache.Repository
	syncer *syncengine.Engine
	model  tui.RootModel
}

func NewRuntime(db *gorm.DB, repo *cache.Repository, syncer *syncengine.Engine, model tui.RootModel) *Runtime {
	return &Runtime{
		db:     db,
		repo:   repo,
		syncer: syncer,
		model:  model,
	}
}

func (r *Runtime) Run(ctx context.Context) error {
	if r.syncer != nil {
		go func() {
			_ = r.syncer.Run(ctx)
		}()
	}
	p := tea.NewProgram(r.model, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
