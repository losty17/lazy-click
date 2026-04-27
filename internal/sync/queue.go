package syncengine

import (
	"time"

	"lazy-click/internal/cache"
	"lazy-click/internal/provider"

	"github.com/google/uuid"
)

func (e *Engine) QueueCreateTask(listID string, task provider.Task) error {
	tempID := "tmp_task_" + uuid.New().String()
	task.ID = tempID
	task.Provider = e.providerKey
	if task.Provider == "" {
		task.Provider = "clickup"
	}

	entity := mapTaskToEntity(task, listID)
	entity.SyncState = cache.SyncStatePendingCreate
	if err := e.repo.SaveTasks([]cache.TaskEntity{entity}); err != nil {
		return err
	}

	payload, err := cache.MarshalPayload(createTaskPayload{
		ListID: listID,
		Task:   task,
	})
	if err != nil {
		return err
	}

	return e.enqueue(opCreateTask, "task", tempID, payload)
}

func (e *Engine) QueueTaskUpdate(taskID string, update provider.TaskUpdate) error {
	// Optimistic update
	if err := e.repo.UpdateTaskSyncState(taskID, cache.SyncStatePendingUpdate, ""); err != nil {
		return err
	}
	// We might want to apply the update to local cache too
	// For now, let's just mark it as pending update.

	payload, err := cache.MarshalPayload(updateTaskPayload{
		TaskID: taskID,
		Update: update,
	})
	if err != nil {
		return err
	}

	return e.enqueue(opUpdateTask, "task", taskID, payload)
}

func (e *Engine) QueueDeleteTask(taskID string) error {
	if err := e.repo.UpdateTaskSyncState(taskID, cache.SyncStatePendingDelete, ""); err != nil {
		return err
	}

	payload, err := cache.MarshalPayload(deleteTaskPayload{
		TaskID: taskID,
	})
	if err != nil {
		return err
	}

	return e.enqueue(opDeleteTask, "task", taskID, payload)
}

func (e *Engine) QueueCreateList(spaceID string, name string) error {
	tempID := "tmp_list_" + uuid.New().String()
	entity := cache.ListEntity{
		ID:         tempID,
		Provider:   e.providerKey,
		ExternalID: tempID,
		SpaceID:    spaceID,
		Name:       name,
		SyncState:  cache.SyncStatePendingCreate,
	}
	if err := e.repo.SaveLists([]cache.ListEntity{entity}); err != nil {
		return err
	}

	payload, err := cache.MarshalPayload(createListPayload{
		SpaceID: spaceID,
		Name:    name,
		LocalID: tempID,
	})
	if err != nil {
		return err
	}

	return e.enqueue(opCreateList, "list", tempID, payload)
}

func (e *Engine) QueueUpdateList(listID string, name string) error {
	if err := e.repo.UpdateListSyncState(listID, cache.SyncStatePendingUpdate, ""); err != nil {
		return err
	}

	payload, err := cache.MarshalPayload(updateListPayload{
		ListID: listID,
		Name:   name,
	})
	if err != nil {
		return err
	}

	return e.enqueue(opUpdateList, "list", listID, payload)
}

func (e *Engine) QueueDeleteList(listID string) error {
	if err := e.repo.UpdateListSyncState(listID, cache.SyncStatePendingDelete, ""); err != nil {
		return err
	}

	payload, err := cache.MarshalPayload(deleteListPayload{
		ListID: listID,
	})
	if err != nil {
		return err
	}

	return e.enqueue(opDeleteList, "list", listID, payload)
}

func (e *Engine) QueueCreateComment(taskID string, text string) error {
	tempID := "tmp_comment_" + uuid.New().String()
	now := time.Now().UnixMilli()
	entity := cache.CommentEntity{
		ID:            tempID,
		TaskID:        taskID,
		BodyMD:        text,
		SyncState:     cache.SyncStatePendingCreate,
		CreatedAtUnix: now,
	}
	if err := e.repo.SaveComments([]cache.CommentEntity{entity}); err != nil {
		return err
	}

	payload, err := cache.MarshalPayload(createCommentPayload{
		TaskID:  taskID,
		Text:    text,
		LocalID: tempID,
	})
	if err != nil {
		return err
	}

	return e.enqueue(opCreateComment, "comment", tempID, payload)
}

func (e *Engine) QueueUpdateComment(commentID string, text string) error {
	if err := e.repo.UpdateCommentSyncState(commentID, cache.SyncStatePendingUpdate, ""); err != nil {
		return err
	}

	payload, err := cache.MarshalPayload(updateCommentPayload{
		CommentID: commentID,
		Text:      text,
	})
	if err != nil {
		return err
	}

	return e.enqueue(opUpdateComment, "comment", commentID, payload)
}

func (e *Engine) QueueDeleteComment(commentID string) error {
	if err := e.repo.UpdateCommentSyncState(commentID, cache.SyncStatePendingDelete, ""); err != nil {
		return err
	}

	payload, err := cache.MarshalPayload(deleteCommentPayload{
		CommentID: commentID,
	})
	if err != nil {
		return err
	}

	return e.enqueue(opDeleteComment, "comment", commentID, payload)
}

func (e *Engine) QueueStartTimeTracking(workspaceID string, taskID string) error {
	payload, err := cache.MarshalPayload(startTimeTrackingPayload{
		WorkspaceID: workspaceID,
		TaskID:      taskID,
	})
	if err != nil {
		return err
	}

	if err := e.enqueue(opStartTimeTracking, "task", taskID, payload); err != nil {
		return err
	}
	e.resetAutomaticSchedule()
	return nil
}

func (e *Engine) QueueStopTimeTracking(workspaceID string) error {
	payload, err := cache.MarshalPayload(stopTimeTrackingPayload{
		WorkspaceID: workspaceID,
	})
	if err != nil {
		return err
	}

	entityID := workspaceID
	if entityID == "" {
		entityID = "global"
	}

	if err := e.enqueue(opStopTimeTracking, "workspace", entityID, payload); err != nil {
		return err
	}
	e.resetAutomaticSchedule()
	return nil
}

func (e *Engine) QueueCreateTimeEntry(workspaceID string, taskID string, entry provider.TimeEntry) error {
	tempID := "tmp_time_" + uuid.New().String()
	entry.ID = tempID

	entity := cache.TimeEntryEntity{
		ID:            tempID,
		Provider:      e.providerKey,
		ExternalID:    tempID,
		TaskID:        taskID,
		Description:   entry.Description,
		StartUnixMS:   entry.StartUnixMS,
		EndUnixMS:     entry.EndUnixMS,
		DurationMS:    entry.DurationMS,
		SyncState:     cache.SyncStatePendingCreate,
		UpdatedAtUnix: time.Now().UnixMilli(),
	}
	if err := e.repo.SaveTimeEntries([]cache.TimeEntryEntity{entity}); err != nil {
		return err
	}

	payload, err := cache.MarshalPayload(createTimeEntryPayload{
		WorkspaceID: workspaceID,
		TaskID:      taskID,
		Entry:       entry,
	})
	if err != nil {
		return err
	}

	return e.enqueue(opCreateTimeEntry, "time_entry", tempID, payload)
}

func (e *Engine) QueueUpdateTimeEntry(workspaceID string, entryID string, update provider.TimeEntryUpdate) error {
	if err := e.repo.UpdateTimeEntrySyncState(entryID, cache.SyncStatePendingUpdate, ""); err != nil {
		return err
	}

	payload, err := cache.MarshalPayload(updateTimeEntryPayload{
		WorkspaceID: workspaceID,
		EntryID:     entryID,
		Update:      update,
	})
	if err != nil {
		return err
	}

	return e.enqueue(opUpdateTimeEntry, "time_entry", entryID, payload)
}

func (e *Engine) QueueDeleteTimeEntry(workspaceID string, entryID string) error {
	if err := e.repo.UpdateTimeEntrySyncState(entryID, cache.SyncStatePendingDelete, ""); err != nil {
		return err
	}

	payload, err := cache.MarshalPayload(deleteTimeEntryPayload{
		WorkspaceID: workspaceID,
		EntryID:     entryID,
	})
	if err != nil {
		return err
	}

	return e.enqueue(opDeleteTimeEntry, "time_entry", entryID, payload)
}

func (e *Engine) enqueue(op string, entityType string, entityID string, payloadJSON string) error {
	now := time.Now().UnixMilli()
	providerID := e.providerKey
	if providerID == "" {
		providerID = "clickup"
	}
	return e.repo.EnqueueSync(cache.SyncQueueEntity{
		Provider:      providerID,
		EntityType:    entityType,
		EntityID:      entityID,
		Operation:     op,
		PayloadJSON:   payloadJSON,
		State:         "pending",
		CreatedAtUnix: now,
		UpdatedAtUnix: now,
	})
}

// Deprecated
func (e *Engine) QueueAddComment(taskID string, text string, localCommentID string) error {
	return e.QueueCreateComment(taskID, text)
}
