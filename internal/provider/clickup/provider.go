package clickup

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"lazy-click/internal/provider"
)

type Provider struct {
	client *Client
}

func New(client *Client) *Provider {
	return &Provider{client: client}
}

func NewFromToken(token string) *Provider {
	return &Provider{
		client: NewClient(token),
	}
}

func (p *Provider) GetSpaces(ctx context.Context) ([]provider.Space, error) {
	resp, err := p.client.GetSpaces(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]provider.Space, 0, len(resp.Spaces))
	for _, s := range resp.Spaces {
		out = append(out, provider.Space{
			ID:   s.ID,
			Name: s.Name,
		})
	}
	return out, nil
}

func (p *Provider) GetLists(ctx context.Context, spaceID string) ([]provider.TaskList, error) {
	resp, err := p.client.GetLists(ctx, spaceID)
	if err != nil {
		return nil, err
	}

	out := make([]provider.TaskList, 0, len(resp.Lists))
	for _, l := range resp.Lists {
		out = append(out, provider.TaskList{
			ID:      l.ID,
			SpaceID: spaceID,
			Name:    l.Name,
		})
	}
	return out, nil
}

func (p *Provider) GetTasks(ctx context.Context, listID string, filter provider.TaskFilter) ([]provider.Task, error) {
	resp, err := p.client.GetTasks(ctx, listID, filter)
	if err != nil {
		return nil, err
	}

	out := make([]provider.Task, 0, len(resp.Tasks))
	for _, t := range resp.Tasks {
		task := provider.Task{
			ID:            t.ID,
			Provider:      "clickup",
			ExternalID:    t.ID,
			ListID:        listID,
			Title:         t.Name,
			DescriptionMD: t.Description,
			Status:        t.Status.Status,
			StatusColor:   t.Status.Color,
			CustomFields:  map[string]any{},
		}
		if t.DueDate != nil {
			if parsedDue, err := strconv.ParseInt(*t.DueDate, 10, 64); err == nil {
				task.DueAtUnixMS = &parsedDue
			}
		}
		if t.TimeEstimate != nil {
			task.EstimateMS = t.TimeEstimate
		}
		if t.Priority != nil {
			rank, _ := strconv.Atoi(t.Priority.OrderIndex)
			task.Priority = &provider.Priority{
				Key:   t.Priority.OrderIndex,
				Label: t.Priority.Priority,
				Rank:  rank,
				Color: t.Priority.Color,
			}
		}
		if t.Parent != nil && *t.Parent != "" {
			task.ParentTaskID = *t.Parent
			task.IsSubtask = true
		}
		for _, tg := range t.Tags {
			task.Tags = append(task.Tags, provider.Tag{
				ID:    tg.Name,
				Name:  tg.Name,
				Color: tg.TagFg,
			})
		}
		for _, assignee := range t.Assignees {
			task.Assignees = append(task.Assignees, provider.User{
				ID:       assignee.ID.String(),
				Provider: "clickup",
				Username: assignee.Username,
				Email:    assignee.Email,
			})
		}
		for _, cf := range t.CustomFields {
			task.CustomFields[cf.Name] = cf.Value
		}

		out = append(out, task)
	}
	return out, nil
}

func (p *Provider) GetTask(ctx context.Context, taskID string) (provider.Task, error) {
	t, err := p.client.GetTask(ctx, taskID)
	if err != nil {
		return provider.Task{}, err
	}

	task := provider.Task{
		ID:            t.ID,
		Provider:      "clickup",
		ExternalID:    t.ID,
		ListID:        t.List.ID,
		Title:         t.Name,
		DescriptionMD: t.Description,
		Status:        t.Status.Status,
		StatusColor:   t.Status.Color,
		CustomFields:  map[string]any{},
	}
	if t.DueDate != nil {
		if parsedDue, parseErr := strconv.ParseInt(*t.DueDate, 10, 64); parseErr == nil {
			task.DueAtUnixMS = &parsedDue
		}
	}
	if t.TimeEstimate != nil {
		task.EstimateMS = t.TimeEstimate
	}
	if t.Priority != nil {
		rank, _ := strconv.Atoi(t.Priority.OrderIndex)
		task.Priority = &provider.Priority{
			Key:   t.Priority.OrderIndex,
			Label: t.Priority.Priority,
			Rank:  rank,
			Color: t.Priority.Color,
		}
	}
	if t.Parent != nil && *t.Parent != "" {
		task.ParentTaskID = *t.Parent
		task.IsSubtask = true
	}
	for _, tg := range t.Tags {
		task.Tags = append(task.Tags, provider.Tag{
			ID:    tg.Name,
			Name:  tg.Name,
			Color: tg.TagFg,
		})
	}
	for _, assignee := range t.Assignees {
		task.Assignees = append(task.Assignees, provider.User{
			ID:       assignee.ID.String(),
			Provider: "clickup",
			Username: assignee.Username,
			Email:    assignee.Email,
		})
	}
	for _, cf := range t.CustomFields {
		task.CustomFields[cf.Name] = cf.Value
	}

	return task, nil
}

func (p *Provider) UpdateTask(ctx context.Context, taskID string, data provider.TaskUpdate) (provider.Task, error) {
	update := UpdateTaskRequest{
		Name:        data.Title,
		Description: data.DescriptionMD,
		Status:      data.Status,
		DueDate:     data.DueAtUnixMS,
	}
	if err := p.client.UpdateTask(ctx, taskID, update); err != nil {
		return provider.Task{}, err
	}

	task, err := p.client.GetTask(ctx, taskID)
	if err != nil {
		return provider.Task{}, err
	}

	mapped, err := p.GetTasks(ctx, task.List.ID, provider.TaskFilter{IncludeClosed: true})
	if err != nil {
		return provider.Task{}, err
	}
	for _, t := range mapped {
		if t.ID == taskID {
			return t, nil
		}
	}
	return provider.Task{}, fmt.Errorf("updated task %s not found after update", taskID)
}

func (p *Provider) AddComment(ctx context.Context, taskID string, text string) (provider.Comment, error) {
	resp, err := p.client.AddComment(ctx, taskID, text)
	if err != nil {
		return provider.Comment{}, err
	}
	return provider.Comment{
		ID:     resp.ID,
		TaskID: taskID,
		Author: provider.User{
			ID:       resp.User.ID.String(),
			Provider: "clickup",
			Username: resp.User.Username,
			Email:    resp.User.Email,
		},
		BodyMD:        decodeCommentText(resp.Comment),
		CreatedAtUnix: parseUnixOrZero(resp.Date),
	}, nil
}

func decodeCommentText(raw json.RawMessage) string {
	var plain string
	if err := json.Unmarshal(raw, &plain); err == nil {
		return plain
	}
	return string(raw)
}

func parseUnixOrZero(value string) int64 {
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0
	}
	return parsed
}
