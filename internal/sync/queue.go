package syncengine

import (
	"time"

	"lazy-click/internal/cache"
	"lazy-click/internal/provider"
)

func (e *Engine) QueueTaskUpdate(taskID string, update provider.TaskUpdate) error {
	payload, err := cache.MarshalPayload(updateTaskPayload{
		TaskID: taskID,
		Update: update,
	})
	if err != nil {
		return err
	}

	now := time.Now().UnixMilli()
	providerID := e.providerKey
	if providerID == "" {
		providerID = "clickup"
	}
	return e.repo.EnqueueSync(cache.SyncQueueEntity{
		Provider:      providerID,
		EntityType:    "task",
		EntityID:      taskID,
		Operation:     opUpdateTask,
		PayloadJSON:   payload,
		State:         "pending",
		CreatedAtUnix: now,
		UpdatedAtUnix: now,
	})
}

func (e *Engine) QueueAddComment(taskID string, text string, localCommentID string) error {
	payload, err := cache.MarshalPayload(addCommentPayload{
		TaskID:         taskID,
		Text:           text,
		LocalCommentID: localCommentID,
	})
	if err != nil {
		return err
	}

	now := time.Now().UnixMilli()
	providerID := e.providerKey
	if providerID == "" {
		providerID = "clickup"
	}
	return e.repo.EnqueueSync(cache.SyncQueueEntity{
		Provider:      providerID,
		EntityType:    "comment",
		EntityID:      taskID,
		Operation:     opAddComment,
		PayloadJSON:   payload,
		State:         "pending",
		CreatedAtUnix: now,
		UpdatedAtUnix: now,
	})
}
