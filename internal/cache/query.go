package cache

import (
	"time"

	"gorm.io/gorm"
)

type TaskListQuery struct {
	ListID        string
	Statuses      []string
	Search        string
	IncludeClosed bool
	Limit         int
	Offset        int
}

func (r *Repository) GetSpaces() ([]SpaceEntity, error) {
	var spaces []SpaceEntity
	err := r.db.Order("name asc").Find(&spaces).Error
	return spaces, err
}

func (r *Repository) GetListsBySpace(spaceID string) ([]ListEntity, error) {
	var lists []ListEntity
	err := r.db.Where("space_id = ?", spaceID).Order("name asc").Find(&lists).Error
	return lists, err
}

func (r *Repository) GetAllLists() ([]ListEntity, error) {
	var lists []ListEntity
	err := r.db.Order("name asc").Find(&lists).Error
	return lists, err
}

func (r *Repository) GetTasksByQuery(q TaskListQuery) ([]TaskEntity, error) {
	stmt := r.db.Model(&TaskEntity{})
	if q.ListID != "" {
		stmt = stmt.Where("list_id = ?", q.ListID)
	}

	if len(q.Statuses) > 0 {
		stmt = stmt.Where("status IN ?", q.Statuses)
	}
	if !q.IncludeClosed {
		stmt = stmt.Where("status <> ?", "closed")
	}
	if q.Search != "" {
		stmt = stmt.Where("title LIKE ? OR description_md LIKE ?", "%"+q.Search+"%", "%"+q.Search+"%")
	}
	if q.Limit > 0 {
		stmt = stmt.Limit(q.Limit).Offset(q.Offset)
	}

	var tasks []TaskEntity
	err := stmt.Order("updated_at_unix desc").Find(&tasks).Error
	return tasks, err
}

func (r *Repository) GetTaskByID(taskID string) (*TaskEntity, error) {
	var task TaskEntity
	err := r.db.Where("id = ?", taskID).First(&task).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &task, nil
}

func (r *Repository) GetTaskComments(taskID string, limit int) ([]CommentEntity, error) {
	stmt := r.db.Where("task_id = ?", taskID).Order("created_at_unix asc")
	if limit > 0 {
		stmt = stmt.Limit(limit)
	}
	var comments []CommentEntity
	err := stmt.Find(&comments).Error
	return comments, err
}

func (r *Repository) NextPendingSyncItem() (*SyncQueueEntity, error) {
	var items []SyncQueueEntity
	err := r.db.Where("state = ?", "pending").Order("created_at_unix asc").Limit(1).Find(&items).Error
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, nil
	}
	return &items[0], nil
}

func (r *Repository) ClaimNextPendingSyncItem() (*SyncQueueEntity, error) {
	item, err := r.NextPendingSyncItem()
	if err != nil || item == nil {
		return item, err
	}

	err = r.db.Model(&SyncQueueEntity{}).
		Where("id = ? AND state = ?", item.ID, "pending").
		Updates(map[string]any{
			"state":           "in_progress",
			"updated_at_unix": time.Now().UnixMilli(),
		}).Error
	if err != nil {
		return nil, err
	}

	var claimed SyncQueueEntity
	if err := r.db.First(&claimed, item.ID).Error; err != nil {
		return nil, err
	}
	return &claimed, nil
}
