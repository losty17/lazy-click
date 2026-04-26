package syncengine

import (
	"context"
	"encoding/json"
	"fmt"

	"lazy-click/internal/cache"
	"lazy-click/internal/provider"
)

const (
	opCreateTask = "create_task"
	opUpdateTask = "update_task"
	opDeleteTask = "delete_task"

	opCreateList = "create_list"
	opUpdateList = "update_list"
	opDeleteList = "delete_list"

	opCreateComment = "create_comment"
	opUpdateComment = "update_comment"
	opDeleteComment = "delete_comment"

	// Deprecated
	opAddComment = "add_comment"
)

type createTaskPayload struct {
	ListID string        `json:"list_id"`
	Task   provider.Task `json:"task"`
}

type updateTaskPayload struct {
	TaskID string              `json:"task_id"`
	Update provider.TaskUpdate `json:"update"`
}

type deleteTaskPayload struct {
	TaskID string `json:"task_id"`
}

type createListPayload struct {
	SpaceID string `json:"space_id"`
	Name    string `json:"name"`
	LocalID string `json:"local_id"`
}

type updateListPayload struct {
	ListID string `json:"list_id"`
	Name   string `json:"name"`
}

type deleteListPayload struct {
	ListID string `json:"list_id"`
}

type createCommentPayload struct {
	TaskID  string `json:"task_id"`
	Text    string `json:"text"`
	LocalID string `json:"local_id"`
}

type updateCommentPayload struct {
	CommentID string `json:"comment_id"`
	Text      string `json:"text"`
}

type deleteCommentPayload struct {
	CommentID string `json:"comment_id"`
}

type addCommentPayload struct {
	TaskID         string `json:"task_id"`
	Text           string `json:"text"`
	LocalCommentID string `json:"local_comment_id,omitempty"`
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
	case opCreateTask:
		var payload createTaskPayload
		if err := json.Unmarshal([]byte(item.PayloadJSON), &payload); err != nil {
			return err
		}
		task, err := e.provider.CreateTask(ctx, payload.ListID, payload.Task)
		if err != nil {
			_ = e.repo.UpdateTaskSyncState(item.EntityID, cache.SyncStateError, err.Error())
			return err
		}
		// Remap temporary ID to real ID
		return e.repo.RemapEntityID(item.EntityID, task.ID, "task")

	case opUpdateTask:
		var payload updateTaskPayload
		if err := json.Unmarshal([]byte(item.PayloadJSON), &payload); err != nil {
			return err
		}
		_, err := e.provider.UpdateTask(ctx, payload.TaskID, payload.Update)
		if err != nil {
			_ = e.repo.UpdateTaskSyncState(payload.TaskID, cache.SyncStateError, err.Error())
			return err
		}
		return e.repo.UpdateTaskSyncState(payload.TaskID, cache.SyncStateSynced, "")

	case opDeleteTask:
		var payload deleteTaskPayload
		if err := json.Unmarshal([]byte(item.PayloadJSON), &payload); err != nil {
			return err
		}
		err := e.provider.DeleteTask(ctx, payload.TaskID)
		if err != nil {
			_ = e.repo.UpdateTaskSyncState(item.EntityID, cache.SyncStateError, err.Error())
			return err
		}
		return e.repo.DeleteTaskByID(payload.TaskID)

	case opCreateList:
		var payload createListPayload
		if err := json.Unmarshal([]byte(item.PayloadJSON), &payload); err != nil {
			return err
		}
		list, err := e.provider.CreateList(ctx, payload.SpaceID, payload.Name)
		if err != nil {
			_ = e.repo.UpdateListSyncState(item.EntityID, cache.SyncStateError, err.Error())
			return err
		}
		return e.repo.RemapEntityID(payload.LocalID, list.ID, "list")

	case opUpdateList:
		var payload updateListPayload
		if err := json.Unmarshal([]byte(item.PayloadJSON), &payload); err != nil {
			return err
		}
		_, err := e.provider.UpdateList(ctx, payload.ListID, payload.Name)
		if err != nil {
			_ = e.repo.UpdateListSyncState(payload.ListID, cache.SyncStateError, err.Error())
			return err
		}
		return e.repo.UpdateListSyncState(payload.ListID, cache.SyncStateSynced, "")

	case opDeleteList:
		var payload deleteListPayload
		if err := json.Unmarshal([]byte(item.PayloadJSON), &payload); err != nil {
			return err
		}
		err := e.provider.DeleteList(ctx, payload.ListID)
		if err != nil {
			_ = e.repo.UpdateListSyncState(item.EntityID, cache.SyncStateError, err.Error())
			return err
		}
		return e.repo.DeleteListByID(payload.ListID)

	case opCreateComment:
		var payload createCommentPayload
		if err := json.Unmarshal([]byte(item.PayloadJSON), &payload); err != nil {
			return err
		}
		comment, err := e.provider.AddComment(ctx, payload.TaskID, payload.Text)
		if err != nil {
			_ = e.repo.UpdateCommentSyncState(item.EntityID, cache.SyncStateError, err.Error())
			return err
		}
		return e.repo.RemapEntityID(payload.LocalID, comment.ID, "comment")

	case opUpdateComment:
		var payload updateCommentPayload
		if err := json.Unmarshal([]byte(item.PayloadJSON), &payload); err != nil {
			return err
		}
		_, err := e.provider.UpdateComment(ctx, payload.CommentID, payload.Text)
		if err != nil {
			_ = e.repo.UpdateCommentSyncState(payload.CommentID, cache.SyncStateError, err.Error())
			return err
		}
		return e.repo.UpdateCommentSyncState(payload.CommentID, cache.SyncStateSynced, "")

	case opDeleteComment:
		var payload deleteCommentPayload
		if err := json.Unmarshal([]byte(item.PayloadJSON), &payload); err != nil {
			return err
		}
		err := e.provider.DeleteComment(ctx, payload.CommentID)
		if err != nil {
			_ = e.repo.UpdateCommentSyncState(item.EntityID, cache.SyncStateError, err.Error())
			return err
		}
		return e.repo.DeleteCommentByID(payload.CommentID)

	case opAddComment:
		var payload addCommentPayload
		if err := json.Unmarshal([]byte(item.PayloadJSON), &payload); err != nil {
			return err
		}
		comment, err := e.provider.AddComment(ctx, payload.TaskID, payload.Text)
		if err != nil {
			return err
		}
		if payload.LocalCommentID != "" {
			_ = e.repo.DeleteCommentByID(payload.LocalCommentID)
		}
		authorName := comment.Author.Username
		if authorName == "" {
			authorName = comment.Author.Email
		}
		return e.repo.SaveComments([]cache.CommentEntity{{
			ID:             comment.ID,
			TaskID:         comment.TaskID,
			AuthorID:       comment.Author.ID,
			AuthorName:     authorName,
			BodyMD:         comment.BodyMD,
			RawPayloadJSON: comment.RawPayloadJSON,
			CreatedAtUnix:  comment.CreatedAtUnix,
			SyncState:      cache.SyncStateSynced,
		}})
	default:
		return fmt.Errorf("unsupported queue operation: %s", item.Operation)
	}
}
