package local

import (
	"context"
	"fmt"
	"time"

	"lazy-click/internal/cache"
	"lazy-click/internal/provider"
)

const (
	ProviderType   = "local"
	defaultSpaceID = "local-space"
	defaultListID  = "local-inbox"
)

type Provider struct {
	repo *cache.Repository
}

func New(repo *cache.Repository) *Provider {
	p := &Provider{repo: repo}
	_ = p.ensureSeedData()
	return p
}

func (p *Provider) ensureSeedData() error {
	if p.repo == nil {
		return fmt.Errorf("cache repository unavailable")
	}
	if err := p.repo.SaveSpaces([]cache.SpaceEntity{{
		ID:         defaultSpaceID,
		Provider:   ProviderType,
		ExternalID: defaultSpaceID,
		Name:       "Local",
	}}); err != nil {
		return err
	}
	if err := p.repo.SaveLists([]cache.ListEntity{{
		ID:         defaultListID,
		Provider:   ProviderType,
		ExternalID: defaultListID,
		SpaceID:    defaultSpaceID,
		Name:       "Inbox",
	}}); err != nil {
		return err
	}
	return nil
}

func (p *Provider) GetCurrentUser(ctx context.Context) (provider.User, error) {
	return provider.User{ID: "local-user", Provider: ProviderType, Username: "you"}, nil
}

func (p *Provider) GetSpaces(ctx context.Context) ([]provider.Space, error) {
	_ = ctx
	if err := p.ensureSeedData(); err != nil {
		return nil, err
	}
	rows, err := p.repo.GetSpacesByProvider(ProviderType)
	if err != nil {
		return nil, err
	}
	out := make([]provider.Space, 0, len(rows))
	for _, row := range rows {
		out = append(out, provider.Space{ID: row.ID, Name: row.Name})
	}
	return out, nil
}

func (p *Provider) GetLists(ctx context.Context, spaceID string) ([]provider.TaskList, error) {
	_ = ctx
	if err := p.ensureSeedData(); err != nil {
		return nil, err
	}
	rows, err := p.repo.GetListsByQuery(cache.ListQuery{Provider: ProviderType, SortMode: cache.ListSortNameAsc})
	if err != nil {
		return nil, err
	}
	out := make([]provider.TaskList, 0, len(rows))
	for _, row := range rows {
		if spaceID != "" && row.SpaceID != spaceID {
			continue
		}
		out = append(out, provider.TaskList{ID: row.ID, SpaceID: row.SpaceID, Name: row.Name})
	}
	return out, nil
}

func (p *Provider) GetTasks(ctx context.Context, listID string, filter provider.TaskFilter) ([]provider.Task, error) {
	_ = ctx
	rows, err := p.repo.GetTasksByQuery(cache.TaskListQuery{
		Provider:      ProviderType,
		ListID:        listID,
		Statuses:      filter.Statuses,
		Search:        filter.Search,
		IncludeClosed: filter.IncludeClosed,
	})
	if err != nil {
		return nil, err
	}
	out := make([]provider.Task, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapEntityToTask(row))
	}
	return out, nil
}

func (p *Provider) GetTask(ctx context.Context, taskID string) (provider.Task, error) {
	_ = ctx
	row, err := p.repo.GetTaskByID(taskID)
	if err != nil {
		return provider.Task{}, err
	}
	if row == nil || row.Provider != ProviderType {
		return provider.Task{}, fmt.Errorf("task %s not found", taskID)
	}
	return mapEntityToTask(*row), nil
}

func (p *Provider) GetTaskComments(ctx context.Context, taskID string) ([]provider.Comment, error) {
	_ = ctx
	rows, err := p.repo.GetTaskComments(taskID, 200)
	if err != nil {
		return nil, err
	}
	out := make([]provider.Comment, 0, len(rows))
	for _, row := range rows {
		out = append(out, provider.Comment{
			ID:     row.ID,
			TaskID: row.TaskID,
			Author: provider.User{
				ID:       row.AuthorID,
				Provider: ProviderType,
				Username: row.AuthorName,
			},
			BodyMD:        row.BodyMD,
			CreatedAtUnix: row.CreatedAtUnix,
		})
	}
	return out, nil
}

func (p *Provider) UpdateTask(ctx context.Context, taskID string, data provider.TaskUpdate) (provider.Task, error) {
	_ = ctx
	row, err := p.repo.GetTaskByID(taskID)
	if err != nil {
		return provider.Task{}, err
	}
	if row == nil || row.Provider != ProviderType {
		return provider.Task{}, fmt.Errorf("task %s not found", taskID)
	}
	next := *row
	if data.Title != nil {
		next.Title = *data.Title
	}
	if data.DescriptionMD != nil {
		next.DescriptionMD = *data.DescriptionMD
	}
	if data.Status != nil {
		next.Status = *data.Status
	}
	if data.DueAtUnixMS != nil {
		next.DueAtUnixMS = data.DueAtUnixMS
	}
	next.UpdatedAtUnix = time.Now().UnixMilli()
	if err := p.repo.SaveTasks([]cache.TaskEntity{next}); err != nil {
		return provider.Task{}, err
	}
	return mapEntityToTask(next), nil
}

func (p *Provider) AddComment(ctx context.Context, taskID string, text string) (provider.Comment, error) {
	_ = ctx
	now := time.Now().UnixMilli()
	comment := cache.CommentEntity{
		ID:            fmt.Sprintf("local-provider-%d", now),
		TaskID:        taskID,
		AuthorID:      "local-user",
		AuthorName:    "you",
		BodyMD:        text,
		CreatedAtUnix: now,
	}
	if err := p.repo.SaveComments([]cache.CommentEntity{comment}); err != nil {
		return provider.Comment{}, err
	}
	return provider.Comment{
		ID:            comment.ID,
		TaskID:        taskID,
		Author:        provider.User{ID: "local-user", Provider: ProviderType, Username: "you"},
		BodyMD:        text,
		CreatedAtUnix: now,
	}, nil
}

func mapEntityToTask(row cache.TaskEntity) provider.Task {
	task := provider.Task{
		ID:            row.ID,
		Provider:      row.Provider,
		ExternalID:    row.ExternalID,
		ListID:        row.ListID,
		ParentTaskID:  row.ParentTaskID,
		IsSubtask:     row.IsSubtask,
		Title:         row.Title,
		DescriptionMD: row.DescriptionMD,
		Status:        row.Status,
		StatusColor:   row.StatusColor,
		EstimateMS:    row.EstimateMS,
		DueAtUnixMS:   row.DueAtUnixMS,
		UpdatedAtUnix: row.UpdatedAtUnix,
		CustomFields:  map[string]any{"source": "local-db"},
	}
	if row.PriorityLabel != "" || row.PriorityKey != "" || row.PriorityRank != 0 {
		task.Priority = &provider.Priority{
			Key:   row.PriorityKey,
			Label: row.PriorityLabel,
			Rank:  row.PriorityRank,
			Color: row.PriorityColor,
		}
	}
	return task
}
