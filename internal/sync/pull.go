package syncengine

import (
	"context"
	"encoding/json"
	"fmt"

	"command-task/internal/cache"
	"command-task/internal/provider"
)

func (e *Engine) PullOnce(ctx context.Context) error {
	spaces, err := e.provider.GetSpaces(ctx)
	if err != nil {
		return err
	}

	spaceRows := make([]cache.SpaceEntity, 0, len(spaces))
	for _, s := range spaces {
		spaceRows = append(spaceRows, cache.SpaceEntity{
			ID:         s.ID,
			Provider:   "clickup",
			ExternalID: s.ID,
			Name:       s.Name,
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
				Provider:   "clickup",
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

		for _, l := range lists {
			tasks, err := e.provider.GetTasks(ctx, l.ID, provider.TaskFilter{IncludeClosed: true})
			if err != nil {
				errorCount++
				lastErr = fmt.Errorf("list %s tasks failed: %w", l.ID, err)
				continue
			}

			taskRows := make([]cache.TaskEntity, 0, len(tasks))
			for _, t := range tasks {
				taskRows = append(taskRows, mapTaskToEntity(t, l.ID))
			}
			if err := e.repo.SaveTasks(taskRows); err != nil {
				errorCount++
				lastErr = fmt.Errorf("list %s task save failed: %w", l.ID, err)
				continue
			}
		}
	}

	if errorCount > 0 {
		return fmt.Errorf("pull completed with %d partial errors (last: %v)", errorCount, lastErr)
	}
	return nil
}

func mapTaskToEntity(task provider.Task, listID string) cache.TaskEntity {
	row := cache.TaskEntity{
		ID:            task.ID,
		Provider:      task.Provider,
		ExternalID:    task.ExternalID,
		ListID:        listID,
		Title:         task.Title,
		DescriptionMD: task.DescriptionMD,
		Status:        task.Status,
		UpdatedAtUnix: task.UpdatedAtUnix,
	}
	if task.Priority != nil {
		row.PriorityKey = task.Priority.Key
		row.PriorityLabel = task.Priority.Label
		row.PriorityRank = task.Priority.Rank
	}
	row.DueAtUnixMS = task.DueAtUnixMS
	if len(task.CustomFields) > 0 {
		if b, err := json.Marshal(task.CustomFields); err == nil {
			row.CustomFieldsJSON = string(b)
		}
	}
	return row
}
