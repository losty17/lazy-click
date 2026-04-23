package cache

import (
	"time"

	"gorm.io/gorm"
)

type TaskListQuery struct {
	Provider      string
	ListID        string
	Statuses      []string
	AssigneeIDs   []string
	Search        string
	IncludeClosed bool
	Limit         int
	Offset        int
}

type ListSortMode string

const (
	ListSortNameAsc          ListSortMode = "name"
	ListSortMostRecentlyOpen ListSortMode = "recent"
)

type ListQuery struct {
	Provider      string
	Search        string
	FavoritesOnly bool
	SortMode      ListSortMode
}

func (r *Repository) GetSpaces() ([]SpaceEntity, error) {
	var spaces []SpaceEntity
	err := r.db.Order("name asc").Find(&spaces).Error
	return spaces, err
}

func (r *Repository) GetSpacesByProvider(provider string) ([]SpaceEntity, error) {
	var spaces []SpaceEntity
	stmt := r.db.Model(&SpaceEntity{})
	if provider != "" {
		stmt = stmt.Where("provider = ?", provider)
	}
	err := stmt.Order("name asc").Find(&spaces).Error
	return spaces, err
}

func (r *Repository) GetListsBySpace(spaceID string) ([]ListEntity, error) {
	var lists []ListEntity
	err := r.db.Where("space_id = ?", spaceID).Order("name asc").Find(&lists).Error
	return lists, err
}

func (r *Repository) GetAllLists() ([]ListEntity, error) {
	return r.GetListsByQuery(ListQuery{SortMode: ListSortNameAsc})
}

func (r *Repository) GetListsByQuery(q ListQuery) ([]ListEntity, error) {
	var lists []ListEntity
	stmt := r.db.Model(&ListEntity{})
	if q.Provider != "" {
		stmt = stmt.Where("provider = ?", q.Provider)
	}
	if q.Search != "" {
		stmt = stmt.Where("name LIKE ?", "%"+q.Search+"%")
	}
	if q.FavoritesOnly {
		stmt = stmt.Where("favorite = ?", true)
	}
	if q.SortMode == ListSortMostRecentlyOpen {
		stmt = stmt.Order("last_opened_unix desc").Order("name asc")
	} else {
		stmt = stmt.Order("name asc")
	}
	err := stmt.Find(&lists).Error
	return lists, err
}

func (r *Repository) GetMostRecentlyOpenedListID() (string, error) {
	var list ListEntity
	err := r.db.Model(&ListEntity{}).
		Where("last_opened_unix > 0").
		Order("last_opened_unix desc").
		Limit(1).
		First(&list).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return "", nil
		}
		return "", err
	}
	return list.ID, nil
}

func (r *Repository) GetTaskStatusesByList(listID string) ([]string, error) {
	stmt := r.db.Model(&TaskEntity{})
	if listID != "" {
		stmt = stmt.Where("list_id = ?", listID)
	}
	var statuses []string
	err := stmt.Where("status <> ''").Distinct().Order("status asc").Pluck("status", &statuses).Error
	return statuses, err
}

func (r *Repository) GetTasksByQuery(q TaskListQuery) ([]TaskEntity, error) {
	stmt := r.db.Model(&TaskEntity{})
	if q.Provider != "" {
		stmt = stmt.Where("provider = ?", q.Provider)
	}
	if q.ListID != "" {
		stmt = stmt.Where("list_id = ?", q.ListID)
	}

	if len(q.Statuses) > 0 {
		stmt = stmt.Where("status IN ?", q.Statuses)
	}
	if len(q.AssigneeIDs) > 0 {
		for _, id := range q.AssigneeIDs {
			// Simplistic JSON search using LIKE. Works for basic user IDs in ClickUp's JSON format.
			stmt = stmt.Where("assignees_json LIKE ?", "%\"id\":\""+id+"\"%")
		}
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

func (r *Repository) NextPendingSyncItem(provider string) (*SyncQueueEntity, error) {
	var items []SyncQueueEntity
	stmt := r.db.Where("state = ?", "pending")
	if provider != "" {
		stmt = stmt.Where("provider = ?", provider)
	}
	err := stmt.Order("created_at_unix asc").Limit(1).Find(&items).Error
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, nil
	}
	return &items[0], nil
}

func (r *Repository) ClaimNextPendingSyncItem(provider string) (*SyncQueueEntity, error) {
	item, err := r.NextPendingSyncItem(provider)
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
