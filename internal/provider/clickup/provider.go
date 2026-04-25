package clickup

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

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

func (p *Provider) GetCurrentUser(ctx context.Context) (provider.User, error) {
	u, err := p.client.GetCurrentUser(ctx)
	if err != nil {
		return provider.User{}, err
	}
	return provider.User{
		ID:       u.ID.String(),
		Provider: "clickup",
		Username: u.Username,
		Email:    u.Email,
	}, nil
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
			DescriptionMD: maybeDecodeRichText(t.Description),
			Status:        t.Status.Status,
			StatusColor:   t.Status.Color,
			CustomFields:  map[string]any{},
		}
		task.ListIDs = append(task.ListIDs, t.List.ID)
		for _, l := range t.Lists {
			task.ListIDs = append(task.ListIDs, l.ID)
		}

		// Ensure the list we are currently fetching from is included
		found := false
		for _, lID := range task.ListIDs {
			if lID == listID {
				found = true
				break
			}
		}
		if !found {
			task.ListIDs = append(task.ListIDs, listID)
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
		for _, att := range t.Attachments {
			task.Attachments = append(task.Attachments, provider.Attachment{
				ID:           att.ID,
				Filename:     att.Title,
				URL:          att.URL,
				ThumbnailURL: att.ThumbnailLarge,
				Size:         att.Size,
				ContentType:  att.Extension,
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
		DescriptionMD: maybeDecodeRichText(t.Description),
		Status:        t.Status.Status,
		StatusColor:   t.Status.Color,
		CustomFields:  map[string]any{},
	}
	task.ListIDs = append(task.ListIDs, t.List.ID)
	for _, l := range t.Lists {
		task.ListIDs = append(task.ListIDs, l.ID)
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
	for _, att := range t.Attachments {
		task.Attachments = append(task.Attachments, provider.Attachment{
			ID:           att.ID,
			Filename:     att.Title,
			URL:          att.URL,
			ThumbnailURL: att.ThumbnailLarge,
			Size:         att.Size,
			ContentType:  att.Extension,
		})
	}
	for _, cf := range t.CustomFields {
		task.CustomFields[cf.Name] = cf.Value
	}

	return task, nil
}

func (p *Provider) GetTaskComments(ctx context.Context, taskID string) ([]provider.Comment, error) {
	resp, err := p.client.GetTaskComments(ctx, taskID)
	if err != nil {
		return nil, err
	}
	out := make([]provider.Comment, 0, len(resp.Comments))
	for _, c := range resp.Comments {
		out = append(out, provider.Comment{
			ID:     c.ID,
			TaskID: taskID,
			Author: provider.User{
				ID:       c.User.ID.String(),
				Provider: "clickup",
				Username: c.User.Username,
				Email:    c.User.Email,
			},
			BodyMD:        decodeCommentText(c.Comment),
			CreatedAtUnix: parseUnixOrZero(c.Date),
		})
	}
	return out, nil
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

func maybeDecodeRichText(s string) string {
	if s == "" {
		return ""
	}
	// If it's a JSON string representing rich text, try to decode it.
	if (strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]")) || (strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}")) {
		return decodeCommentText(json.RawMessage(s))
	}
	return s
}

func decodeCommentText(raw json.RawMessage) string {
	var plain string
	if err := json.Unmarshal(raw, &plain); err == nil {
		return plain
	}

	// Try to parse as ClickUp rich text (array of objects) or Quill Delta (ops)
	var rich []map[string]any

	// Check if it's Quill Delta format { "ops": [...] }
	var delta struct {
		Ops []map[string]any `json:"ops"`
	}
	if err := json.Unmarshal(raw, &delta); err == nil && len(delta.Ops) > 0 {
		rich = delta.Ops
	} else if err := json.Unmarshal(raw, &rich); err != nil {
		return string(raw)
	}

	var buf strings.Builder
	for _, part := range rich {
		// Handle Quill Delta 'insert' style
		content := part
		if insert, ok := part["insert"]; ok {
			if s, ok := insert.(string); ok {
				buf.WriteString(s)
				continue
			}
			if m, ok := insert.(map[string]any); ok {
				content = m
			}
		}

		if text, ok := content["text"].(string); ok && text != "" {
			buf.WriteString(text)
			continue
		}

		// Handle Mentions
		if mentionType, ok := content["type"].(string); ok && mentionType == "mention" {
			if user, ok := content["user"].(map[string]any); ok {
				if username, ok := user["username"].(string); ok {
					buf.WriteString("@" + username)
					continue
				}
			}
		}

		// Handle Embeds
		if embed, ok := content["embed"]; ok {
			url := ""
			if s, ok := content["url"].(string); ok {
				url = s
			} else if s, ok := content["link"].(string); ok {
				url = s
			} else if m, ok := embed.(map[string]any); ok {
				if s, ok := m["url"].(string); ok {
					url = s
				} else if s, ok := m["link"].(string); ok {
					url = s
				}
			}
			if url != "" {
				buf.WriteString(fmt.Sprintf("[Embed: %s]", url))
			} else {
				buf.WriteString("[Embed]")
			}
			continue
		}

		// Handle Attachments
		if attachment, ok := content["attachment"]; ok {
			url := ""
			if s, ok := content["url"].(string); ok {
				url = s
			} else if s, ok := content["link"].(string); ok {
				url = s
			} else if m, ok := attachment.(map[string]any); ok {
				if s, ok := m["url"].(string); ok {
					url = s
				} else if s, ok := m["link"].(string); ok {
					url = s
				}
			}
			if url != "" {
				buf.WriteString(fmt.Sprintf("[Attachment: %s]", url))
			} else {
				buf.WriteString("[Attachment]")
			}
			continue
		}

		// Quill Delta video/image embeds
		if video, ok := content["video"].(string); ok {
			buf.WriteString(fmt.Sprintf("[Video: %s]", video))
			continue
		}
		if image, ok := content["image"].(string); ok {
			buf.WriteString(fmt.Sprintf("[Image: %s]", image))
			continue
		}
	}
	return buf.String()
}

func parseUnixOrZero(value string) int64 {
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0
	}
	return parsed
}
