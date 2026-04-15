package syncengine

import (
	"time"

	"command-task/internal/cache"
	"command-task/internal/provider"
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
	return e.repo.EnqueueSync(cache.SyncQueueEntity{
		Provider:      "clickup",
		EntityType:    "task",
		EntityID:      taskID,
		Operation:     opUpdateTask,
		PayloadJSON:   payload,
		State:         "pending",
		CreatedAtUnix: now,
		UpdatedAtUnix: now,
	})
}

func (e *Engine) QueueAddComment(taskID string, text string) error {
	payload, err := cache.MarshalPayload(addCommentPayload{
		TaskID: taskID,
		Text:   text,
	})
	if err != nil {
		return err
	}

	now := time.Now().UnixMilli()
	return e.repo.EnqueueSync(cache.SyncQueueEntity{
		Provider:      "clickup",
		EntityType:    "comment",
		EntityID:      taskID,
		Operation:     opAddComment,
		PayloadJSON:   payload,
		State:         "pending",
		CreatedAtUnix: now,
		UpdatedAtUnix: now,
	})
}
