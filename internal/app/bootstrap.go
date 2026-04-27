package app

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"lazy-click/internal/cache"
	"lazy-click/internal/config"
	"lazy-click/internal/provider/clickup"
	"lazy-click/internal/provider/local"
	syncengine "lazy-click/internal/sync"
	"lazy-click/internal/tui"
	"lazy-click/internal/attachment"
)

const appStateClickUpToken = "provider.clickup.pat"
const appStateActiveProviderID = "ui.active_provider_id"

func Bootstrap(ctx context.Context) (*Runtime, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(cfg.DBPath), 0o755); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}

	db, err := cache.Open(cfg.DBPath)
	if err != nil {
		return nil, err
	}

	repo := cache.NewRepository(db)

	attachmentDir := filepath.Join(filepath.Dir(cfg.DBPath), "attachments")
	attManager, err := attachment.NewManager(attachmentDir)
	if err != nil {
		return nil, fmt.Errorf("create attachment manager: %w", err)
	}

	logger := log.New(os.Stderr, "sync: ", log.LstdFlags)
	localProvider := local.New(repo)
	localEngine := syncengine.NewEngine(repo, local.ProviderType, localProvider, logger, 15*time.Second)

	clickupToken := strings.TrimSpace(cfg.ClickUpAPIToken)
	if clickupToken == "" {
		if tokenFromState, stateErr := repo.GetAppState(appStateClickUpToken); stateErr == nil {
			clickupToken = strings.TrimSpace(tokenFromState)
		}
	}
	clickupProvider := clickup.NewFromToken(clickupToken)
	clickupEngine := syncengine.NewEngine(repo, "clickup", clickupProvider, logger, 15*time.Second)

	activeProviderID := strings.TrimSpace(cfg.DefaultProvider)
	if savedProviderID, stateErr := repo.GetAppState(appStateActiveProviderID); stateErr == nil && strings.TrimSpace(savedProviderID) != "" {
		activeProviderID = strings.TrimSpace(savedProviderID)
	}

	router := syncengine.NewRouter([]syncengine.ProviderNode{
		syncengine.BuildProviderNode(syncengine.ProviderMeta{ID: "local", Kind: "local", DisplayName: "Local"}, localEngine, localProvider),
		syncengine.BuildProviderNode(syncengine.ProviderMeta{
			ID:                "clickup",
			Kind:              "clickup",
			DisplayName:       "ClickUp",
			TokenInstructions: "Settings > ClickUp API > API Token",
		}, clickupEngine, clickupProvider),
	}, activeProviderID)

	statusLine := ""
	if strings.TrimSpace(clickupToken) == "" {
		statusLine = "ClickUp is available but not connected; run control center command: Set ClickUp Personal Access Token"
	}
	clickUpConnected := strings.TrimSpace(clickupToken) != ""

	syncers := []*syncengine.Engine{localEngine}
	if strings.TrimSpace(clickupToken) != "" {
		syncers = append(syncers, clickupEngine)
	}
	needsProviderSetup := false
	if strings.TrimSpace(cfg.DefaultProvider) == "" && strings.TrimSpace(activeProviderID) == "" {
		needsProviderSetup = true
	}

	model := tui.NewRootModel(repo, router, attManager, router.ProviderDisplayName(), statusLine, clickUpConnected, needsProviderSetup)
	return NewRuntime(db, repo, syncers, model), nil
}
