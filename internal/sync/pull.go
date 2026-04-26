package syncengine

import (
	"context"
	"encoding/json"
	"fmt"

	"lazy-click/internal/cache"
	"lazy-click/internal/provider"
)

func (e *Engine) PullOnce(ctx context.Context) error {
	if err := e.PullMetadataOnce(ctx); err != nil {
		return err
	}
	activeList := e.ActiveListID()
	if activeList == "" {
		return nil
	}
	return e.PullListTasksOnce(ctx, activeList)
}

func (e *Engine) PullMetadataOnce(ctx context.Context) error {
	e.setSyncStatus("fetching spaces")
	spaces, err := e.provider.GetSpaces(ctx)
	if err != nil {
		return err
	}

	spaceRows := make([]cache.SpaceEntity, 0, len(spaces))
	for _, s := range spaces {
		spaceRows = append(spaceRows, cache.SpaceEntity{
			ID:            s.ID,
			Provider:      e.providerKey,
			ExternalID:    s.ID,
			WorkspaceID:   s.WorkspaceID,
			WorkspaceName: s.WorkspaceName,
			Name:          s.Name,
		})
	}
	if err := e.repo.SaveSpaces(spaceRows); err != nil {
		return err
	}

	var (
		errorCount int
		lastErr    error
	)

	for _, s := range spaces {
		e.setSyncStatus("fetching lists for space " + s.ID)
		lists, err := e.provider.GetLists(ctx, s.ID)
		if err != nil {
			errorCount++
			lastErr = fmt.Errorf("space %s lists failed: %w", s.ID, err)
			continue
		}

		listRows := make([]cache.ListEntity, 0, len(lists))
		for _, l := range lists {
			listRows = append(listRows, cache.ListEntity{
				ID:         l.ID,
				Provider:   e.providerKey,
				ExternalID: l.ID,
				SpaceID:    s.ID,
				Name:       l.Name,
			})
		}
		if err := e.repo.SaveLists(listRows); err != nil {
			errorCount++
			lastErr = fmt.Errorf("space %s list save failed: %w", s.ID, err)
			continue
		}

	}

	if errorCount > 0 {
		return fmt.Errorf("pull completed with %d partial errors (last: %v)", errorCount, lastErr)
	}
	return nil
}

func (e *Engine) PullListTasksOnce(ctx context.Context, listID string) error {
	if listID == "" {
		return nil
	}
	e.setSyncStatus("fetching tasks for list " + listID)
	tasks, err := e.provider.GetTasks(ctx, listID, provider.TaskFilter{IncludeClosed: true})
	if err != nil {
		return fmt.Errorf("list %s tasks failed: %w", listID, err)
	}
	taskRows := make([]cache.TaskEntity, 0, len(tasks))
	for _, t := range tasks {
		taskRows = append(taskRows, mapTaskToEntity(t, listID))
	}
	if err := e.repo.SaveTasks(taskRows); err != nil {
		return fmt.Errorf("list %s task save failed: %w", listID, err)
	}
	if err := e.repo.MarkListSynced(listID); err != nil {
		return fmt.Errorf("list %s sync marker failed: %w", listID, err)
	}
	return nil
}

func mapTaskToEntity(task provider.Task, listID string) cache.TaskEntity {
	row := cache.TaskEntity{
		ID:            task.ID,
		Provider:      task.Provider,
		ExternalID:    task.ExternalID,
		ListID:        listID,
		ParentTaskID:  task.ParentTaskID,
		IsSubtask:     task.IsSubtask,
		Title:         task.Title,
		DescriptionMD: task.DescriptionMD,
		Status:        task.Status,
		StatusColor:   task.StatusColor,
		UpdatedAtUnix: task.UpdatedAtUnix,
		ListIDs:       task.ListIDs,
	}
	if task.Priority != nil {
		row.PriorityKey = task.Priority.Key
		row.PriorityLabel = task.Priority.Label
		row.PriorityRank = task.Priority.Rank
		row.PriorityColor = task.Priority.Color
	}
	row.EstimateMS = task.EstimateMS
	row.DueAtUnixMS = task.DueAtUnixMS
	if len(task.CustomFields) > 0 {
		if b, err := json.Marshal(task.CustomFields); err == nil {
			row.CustomFieldsJSON = string(b)
		}
	}
	if len(task.Assignees) > 0 {
		if b, err := json.Marshal(task.Assignees); err == nil {
			row.AssigneesJSON = string(b)
		}
	}
	if len(task.Attachments) > 0 {
		if b, err := json.Marshal(task.Attachments); err == nil {
			row.AttachmentsJSON = string(b)
		}
	}
	return row
}
