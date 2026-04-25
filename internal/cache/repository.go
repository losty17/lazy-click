package cache

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) DB() *gorm.DB {
	return r.db
}

func (r *Repository) SaveSpaces(spaces []SpaceEntity) error {
	if len(spaces) == 0 {
		return nil
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		for _, space := range spaces {
			if err := tx.Save(&space).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *Repository) SaveLists(lists []ListEntity) error {
	if len(lists) == 0 {
		return nil
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		for _, list := range lists {
			if err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "id"}},
				DoUpdates: clause.AssignmentColumns([]string{"provider", "external_id", "space_id", "name"}),
			}).Create(&list).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *Repository) SaveTasks(tasks []TaskEntity) error {
	if len(tasks) == 0 {
		return nil
	}
	now := time.Now().UnixMilli()
	return r.db.Transaction(func(tx *gorm.DB) error {
		for _, task := range tasks {
			task.LastFetchedUnix = now
			task.UpdatedAt = time.Now()

			// Use Upsert logic to avoid overwriting rich data (Description, Attachments)
			// with empty values if they were fetched by a more detailed API call (like GetTask)
			// but are missing in the current list sync.
			updateColumns := []string{
				"provider", "external_id", "list_id", "parent_task_id", "is_subtask",
				"title", "status", "status_color", "priority_key", "priority_label",
				"priority_rank", "priority_color", "estimate_ms", "due_at_unix_ms",
				"assignees_json", "custom_fields_json", "updated_at_unix", "last_fetched_unix",
				"updated_at",
			}

			// Only update description/attachments if the incoming task has them.
			// This prevents the 'GetTasks' (list) call from clearing data fetched by 'GetTask' (detail).
			if task.DescriptionMD != "" {
				updateColumns = append(updateColumns, "description_md")
			}
			if task.AttachmentsJSON != "" {
				updateColumns = append(updateColumns, "attachments_json")
			}

			if err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "id"}},
				DoUpdates: clause.AssignmentColumns(updateColumns),
			}).Create(&task).Error; err != nil {
				return err
			}

			allListIDs := task.ListIDs
			if len(allListIDs) == 0 && task.ListID != "" {
				allListIDs = []string{task.ListID}
			}

			for _, lID := range allListIDs {
				if lID == "" {
					continue
				}
				join := TaskListJoinEntity{
					TaskID: task.ID,
					ListID: lID,
				}
				if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&join).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (r *Repository) MarkListOpened(listID string) error {
	if listID == "" {
		return nil
	}
	now := time.Now().UnixMilli()
	return r.db.Model(&ListEntity{}).Where("id = ?", listID).Updates(map[string]any{
		"last_opened_unix": now,
		"updated_at":       time.Now(),
	}).Error
}

func (r *Repository) MarkListSynced(listID string) error {
	if listID == "" {
		return nil
	}
	now := time.Now().UnixMilli()
	return r.db.Model(&ListEntity{}).Where("id = ?", listID).Updates(map[string]any{
		"last_synced_unix": now,
		"updated_at":       time.Now(),
	}).Error
}

func (r *Repository) ToggleListFavorite(listID string) error {
	if listID == "" {
		return nil
	}
	var list ListEntity
	if err := r.db.Where("id = ?", listID).First(&list).Error; err != nil {
		return err
	}
	return r.db.Model(&ListEntity{}).Where("id = ?", listID).Updates(map[string]any{
		"favorite":   !list.Favorite,
		"updated_at": time.Now(),
	}).Error
}

func (r *Repository) SetListFavorite(listID string, favorite bool) error {
	if listID == "" {
		return nil
	}
	return r.db.Model(&ListEntity{}).Where("id = ?", listID).Updates(map[string]any{
		"favorite":   favorite,
		"updated_at": time.Now(),
	}).Error
}

func (r *Repository) GetListByID(listID string) (*ListEntity, error) {
	var list ListEntity
	err := r.db.Where("id = ?", listID).First(&list).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &list, nil
}

func (r *Repository) SaveAppState(key string, value string) error {
	if key == "" {
		return fmt.Errorf("app state key is required")
	}
	return r.db.Save(&AppStateEntity{Key: key, Value: value}).Error
}

func (r *Repository) GetAppState(key string) (string, error) {
	if key == "" {
		return "", fmt.Errorf("app state key is required")
	}
	var row AppStateEntity
	err := r.db.Where("key = ?", key).First(&row).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return "", nil
		}
		return "", err
	}
	return row.Value, nil
}

func (r *Repository) EnqueueSync(item SyncQueueEntity) error {
	if item.Provider == "" || item.EntityType == "" || item.EntityID == "" || item.Operation == "" || item.PayloadJSON == "" {
		return errors.New("invalid sync queue item")
	}
	return r.db.Create(&item).Error
}

func (r *Repository) MarkSyncDone(id uint64) error {
	return r.db.Model(&SyncQueueEntity{}).Where("id = ?", id).Updates(map[string]any{
		"state":           "done",
		"last_error":      "",
		"updated_at_unix": time.Now().UnixMilli(),
	}).Error
}

func (r *Repository) MarkSyncFailed(id uint64, message string) error {
	return r.db.Model(&SyncQueueEntity{}).Where("id = ?", id).Updates(map[string]any{
		"state":           "pending",
		"last_error":      message,
		"retry_count":     gorm.Expr("retry_count + 1"),
		"updated_at_unix": time.Now().UnixMilli(),
	}).Error
}

func (r *Repository) SaveComments(comments []CommentEntity) error {
	if len(comments) == 0 {
		return nil
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		for _, comment := range comments {
			if err := tx.Save(&comment).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *Repository) DeleteCommentByID(commentID string) error {
	if commentID == "" {
		return nil
	}
	return r.db.Delete(&CommentEntity{}, "id = ?", commentID).Error
}

func MarshalPayload(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (r *Repository) UpdateTaskTitle(taskID string, title string) error {
	return r.db.Model(&TaskEntity{}).Where("id = ?", taskID).Updates(map[string]any{
		"title":           title,
		"updated_at_unix": time.Now().UnixMilli(),
	}).Error
}
