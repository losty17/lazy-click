package syncengine

import (
	"context"
	"encoding/json"
	"fmt"

	"lazy-click/internal/cache"
	"lazy-click/internal/provider"
)

const (
	opUpdateTask = "update_task"
	opAddComment = "add_comment"
)

type updateTaskPayload struct {
	TaskID string              `json:"task_id"`
	Update provider.TaskUpdate `json:"update"`
}

type addCommentPayload struct {
	TaskID string `json:"task_id"`
	Text   string `json:"text"`
}

func (e *Engine) PushOnce(ctx context.Context) error {
	e.setSyncStatus("checking pending push queue")
	item, err := e.repo.ClaimNextPendingSyncItem(e.providerKey)
	if err != nil || item == nil {
		e.setSyncStatus("no pending push items")
		return err
	}
	e.setSyncStatus("pushing " + item.Operation + " for " + item.EntityType + " " + item.EntityID)

	if err := e.applyQueueItem(ctx, *item); err != nil {
		e.setSyncStatus("push failed for queue item")
		_ = e.repo.MarkSyncFailed(item.ID, err.Error())
		return err
	}
	e.setSyncStatus("push completed for queue item")
	return e.repo.MarkSyncDone(item.ID)
}

func (e *Engine) applyQueueItem(ctx context.Context, item cache.SyncQueueEntity) error {
	switch item.Operation {
	case opUpdateTask:
		var payload updateTaskPayload
		if err := json.Unmarshal([]byte(item.PayloadJSON), &payload); err != nil {
			return err
		}
		_, err := e.provider.UpdateTask(ctx, payload.TaskID, payload.Update)
		return err
	case opAddComment:
		var payload addCommentPayload
		if err := json.Unmarshal([]byte(item.PayloadJSON), &payload); err != nil {
			return err
		}
		_, err := e.provider.AddComment(ctx, payload.TaskID, payload.Text)
		return err
	default:
		return fmt.Errorf("unsupported queue operation: %s", item.Operation)
	}
}
