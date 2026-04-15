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
	item, err := e.repo.ClaimNextPendingSyncItem()
	if err != nil || item == nil {
		return err
	}

	if err := e.applyQueueItem(ctx, *item); err != nil {
		_ = e.repo.MarkSyncFailed(item.ID, err.Error())
		return err
	}
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
