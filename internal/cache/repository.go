package cache

import (
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"
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
			if err := tx.Save(&list).Error; err != nil {
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
	return r.db.Transaction(func(tx *gorm.DB) error {
		for _, task := range tasks {
			if err := tx.Save(&task).Error; err != nil {
				return err
			}
		}
		return nil
	})
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
