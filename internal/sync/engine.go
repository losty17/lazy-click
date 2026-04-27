package syncengine

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"lazy-click/internal/cache"
	"lazy-click/internal/provider"
)

type Engine struct {
	repo        *cache.Repository
	provider    provider.ProjectProvider
	providerKey string
	logger      *log.Logger
	interval    time.Duration

	mu              sync.RWMutex
	activeListID    string
	syncStatus      string
	resetAutoSyncCh chan struct{}
}

func NewEngine(repo *cache.Repository, providerKey string, provider provider.ProjectProvider, logger *log.Logger, interval time.Duration) *Engine {
	if interval <= 0 {
		interval = 10 * time.Second
	}
	return &Engine{
		repo:            repo,
		provider:        provider,
		providerKey:     providerKey,
		logger:          logger,
		interval:        interval,
		syncStatus:      "idle",
		resetAutoSyncCh: make(chan struct{}, 1),
	}
}

func (e *Engine) ProviderKey() string {
	return e.providerKey
}

func (e *Engine) GetCurrentUser(ctx context.Context) (provider.User, error) {
	e.mu.RLock()
	p := e.provider
	e.mu.RUnlock()
	if p == nil {
		return provider.User{}, fmt.Errorf("provider not set")
	}
	return p.GetCurrentUser(ctx)
}

func (e *Engine) SetActiveProvider(providerID string) bool {
	return providerID == "" || providerID == e.providerKey
}

func (e *Engine) ActiveProviderID() string {
	return e.providerKey
}

func (e *Engine) SetProviderAPI(api provider.ProjectProvider) {
	if api == nil {
		return
	}
	e.mu.Lock()
	e.provider = api
	e.mu.Unlock()
}

func (e *Engine) Run(ctx context.Context) error {
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			if err := e.Cycle(ctx); err != nil && e.logger != nil {
				e.logger.Printf("sync cycle failed: %v", err)
			}
			timer.Reset(e.interval)
		case <-e.resetAutoSyncCh:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(e.interval)
		}
	}
}

func (e *Engine) Cycle(ctx context.Context) error {
	e.setSyncStatus("pulling metadata")
	if err := e.PullMetadataOnce(ctx); err != nil {
		e.setSyncStatus("metadata pull failed")
		return err
	}
	active := e.ActiveListID()
	if active != "" {
		e.setSyncStatus("pulling tasks for active list " + active)
		if err := e.PullListTasksOnce(ctx, active); err != nil {
			e.setSyncStatus("active-list task pull failed")
			return err
		}
	}
	e.setSyncStatus("pushing pending mutations")
	if err := e.PushOnce(ctx); err != nil {
		e.setSyncStatus("push failed")
		return err
	}
	e.setSyncStatus("idle")
	return nil
}

func (e *Engine) SetActiveListID(listID string) {
	e.mu.Lock()
	e.activeListID = listID
	e.mu.Unlock()
}

func (e *Engine) ActiveListID() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.activeListID
}

func (e *Engine) SyncList(ctx context.Context, listID string) error {
	defer e.resetAutomaticSchedule()
	e.setSyncStatus("pulling metadata")
	if err := e.PullMetadataOnce(ctx); err != nil {
		e.setSyncStatus("metadata pull failed")
		return err
	}
	if listID != "" {
		e.setSyncStatus("pulling tasks for selected list " + listID)
		if err := e.PullListTasksOnce(ctx, listID); err != nil {
			e.setSyncStatus("selected-list task pull failed")
			return err
		}
	}
	e.setSyncStatus("pushing pending mutations")
	if err := e.PushOnce(ctx); err != nil {
		e.setSyncStatus("push failed")
		return err
	}
	e.setSyncStatus("idle")
	return nil
}

func (e *Engine) RevalidateTask(ctx context.Context, taskID string) error {
	if taskID == "" {
		return nil
	}
	e.setSyncStatus("revalidating task " + taskID)
	task, err := e.provider.GetTask(ctx, taskID)
	if err != nil {
		e.setSyncStatus("task revalidate failed")
		return err
	}
	if task.ListID == "" {
		cached, cacheErr := e.repo.GetTaskByID(taskID)
		if cacheErr != nil {
			e.setSyncStatus("task revalidate failed")
			return cacheErr
		}
		if cached == nil || cached.ListID == "" {
			e.setSyncStatus("task revalidated")
			return nil
		}
		task.ListID = cached.ListID
	}
	if err := e.repo.SaveTasks([]cache.TaskEntity{mapTaskToEntity(task, task.ListID)}); err != nil {
		e.setSyncStatus("task revalidate failed")
		return err
	}
	if err := e.syncTaskComments(ctx, taskID); err != nil {
		e.setSyncStatus("task comments sync failed")
		return err
	}
	if err := e.syncTaskTimeEntries(ctx, taskID); err != nil {
		e.setSyncStatus("task time entries sync failed")
		return err
	}
	e.setSyncStatus("task revalidated")
	return nil
}

func (e *Engine) syncTaskTimeEntries(ctx context.Context, taskID string) error {
	if taskID == "" {
		return nil
	}
	workspaceID, _ := e.repo.GetWorkspaceIDForTask(taskID)
	entries, err := e.provider.GetTimeEntries(ctx, workspaceID, taskID)
	if err != nil {
		return err
	}
	rows := make([]cache.TimeEntryEntity, 0, len(entries))
	for _, entry := range entries {
		rows = append(rows, cache.TimeEntryEntity{
			ID:            entry.ID,
			Provider:      e.providerKey,
			ExternalID:    entry.ID,
			TaskID:        taskID,
			Description:   entry.Description,
			StartUnixMS:   entry.StartUnixMS,
			EndUnixMS:     entry.EndUnixMS,
			DurationMS:    entry.DurationMS,
			SyncState:     cache.SyncStateSynced,
			UpdatedAtUnix: time.Now().UnixMilli(),
		})
	}
	if err := e.repo.SaveTimeEntries(rows); err != nil {
		return err
	}
	return nil
}

func (e *Engine) syncTaskComments(ctx context.Context, taskID string) error {
	if taskID == "" {
		return nil
	}
	comments, err := e.provider.GetTaskComments(ctx, taskID)
	if err != nil {
		return err
	}
	rows := make([]cache.CommentEntity, 0, len(comments))
	for _, comment := range comments {
		authorName := comment.Author.Username
		if authorName == "" {
			authorName = comment.Author.Email
		}
		rows = append(rows, cache.CommentEntity{
			ID:             comment.ID,
			TaskID:         taskID,
			AuthorID:       comment.Author.ID,
			AuthorName:     authorName,
			BodyMD:         comment.BodyMD,
			RawPayloadJSON: comment.RawPayloadJSON,
			CreatedAtUnix:  comment.CreatedAtUnix,
		})
	}
	if err := e.repo.SaveComments(rows); err != nil {
		return err
	}
	return nil
}

func (e *Engine) SyncStatus() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.syncStatus == "" {
		return "idle"
	}
	return e.syncStatus
}

func (e *Engine) setSyncStatus(status string) {
	e.mu.Lock()
	e.syncStatus = status
	e.mu.Unlock()
}

func (e *Engine) resetAutomaticSchedule() {
	select {
	case e.resetAutoSyncCh <- struct{}{}:
	default:
	}
}
