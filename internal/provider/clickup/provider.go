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
			ID:            s.ID.String(),
			Name:          s.Name,
			WorkspaceID:   s.TeamID,
			WorkspaceName: s.TeamName,
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
			ID:      l.ID.String(),
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
		task := p.mapTask(t, listID)

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

		out = append(out, task)
	}
	return out, nil
}

func (p *Provider) GetTask(ctx context.Context, taskID string) (provider.Task, error) {
	t, err := p.client.GetTask(ctx, taskID)
	if err != nil {
		return provider.Task{}, err
	}

	return p.mapTask(*t, t.List.ID.String()), nil
}

func (p *Provider) GetTaskComments(ctx context.Context, taskID string) ([]provider.Comment, error) {
	resp, err := p.client.GetTaskComments(ctx, taskID)
	if err != nil {
		return nil, err
	}
	out := make([]provider.Comment, 0, len(resp.Comments))
	for _, c := range resp.Comments {
		out = append(out, provider.Comment{
			ID:     c.ID.String(),
			TaskID: taskID,
			Author: provider.User{
				ID:       c.User.ID.String(),
				Provider: "clickup",
				Username: c.User.Username,
				Email:    c.User.Email,
			},
			BodyMD:         decodeCommentText(c.Comment),
			RawPayloadJSON: string(c.Comment),
			CreatedAtUnix:  parseUnixOrZero(c.Date.String()),
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
	if data.PriorityKey != nil {
		if prio, err := strconv.Atoi(*data.PriorityKey); err == nil {
			update.Priority = &prio
		}
	}
	if err := p.client.UpdateTask(ctx, taskID, update); err != nil {
		return provider.Task{}, err
	}

	task, err := p.client.GetTask(ctx, taskID)
	if err != nil {
		return provider.Task{}, err
	}

	mapped, err := p.GetTasks(ctx, task.List.ID.String(), provider.TaskFilter{IncludeClosed: true})
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
		ID:     resp.ID.String(),
		TaskID: taskID,
		Author: provider.User{
			ID:       resp.User.ID.String(),
			Provider: "clickup",
			Username: resp.User.Username,
			Email:    resp.User.Email,
		},
		BodyMD:         decodeCommentText(resp.Comment),
		RawPayloadJSON: string(resp.Comment),
		CreatedAtUnix:  parseUnixOrZero(resp.Date.String()),
	}, nil
}

func (p *Provider) CreateTask(ctx context.Context, listID string, task provider.Task) (provider.Task, error) {
	req := CreateTaskRequest{
		Name:        task.Title,
		Description: task.DescriptionMD,
		Status:      task.Status,
		DueDate:     task.DueAtUnixMS,
	}
	if task.Priority != nil {
		prio := task.Priority.Rank
		req.Priority = &prio
	}
	if task.ParentTaskID != "" {
		req.Parent = &task.ParentTaskID
	}

	resp, err := p.client.CreateTask(ctx, listID, req)
	if err != nil {
		return provider.Task{}, err
	}

	return p.mapTask(*resp, listID), nil
}

func (p *Provider) DeleteTask(ctx context.Context, taskID string) error {
	return p.client.DeleteTask(ctx, taskID)
}

func (p *Provider) CreateList(ctx context.Context, spaceID string, name string) (provider.TaskList, error) {
	resp, err := p.client.CreateList(ctx, spaceID, CreateListRequest{Name: name})
	if err != nil {
		return provider.TaskList{}, err
	}
	return provider.TaskList{
		ID:      resp.ID.String(),
		SpaceID: spaceID,
		Name:    resp.Name,
	}, nil
}

func (p *Provider) UpdateList(ctx context.Context, listID string, name string) (provider.TaskList, error) {
	resp, err := p.client.UpdateList(ctx, listID, UpdateListRequest{Name: name})
	if err != nil {
		return provider.TaskList{}, err
	}
	return provider.TaskList{
		ID:   resp.ID.String(),
		Name: resp.Name,
	}, nil
}

func (p *Provider) DeleteList(ctx context.Context, listID string) error {
	return p.client.DeleteList(ctx, listID)
}

func (p *Provider) UpdateComment(ctx context.Context, commentID string, text string) (provider.Comment, error) {
	if err := p.client.UpdateComment(ctx, commentID, text); err != nil {
		return provider.Comment{}, err
	}
	// ClickUp Update Comment doesn't return the full comment object, so we might need to fetch it if we want the updated state.
	// For now, let's just return a partial comment or fetch if necessary.
	// Since we don't have a GetComment API in the client yet, let's assume success is enough for now or return the input.
	return provider.Comment{ID: commentID, BodyMD: text}, nil
}

func (p *Provider) DeleteComment(ctx context.Context, commentID string) error {
	return p.client.DeleteComment(ctx, commentID)
}

func (p *Provider) StartTimeTracking(ctx context.Context, workspaceID string, taskID string) error {
	if workspaceID == "" {
		teams, err := p.client.getTeams(ctx)
		if err != nil {
			return err
		}
		if len(teams.Teams) == 0 {
			return fmt.Errorf("no ClickUp teams found")
		}
		workspaceID = teams.Teams[0].ID.String() // Use first team as default
	}
	return p.client.StartTimeTracking(ctx, workspaceID, taskID)
}

func (p *Provider) StopTimeTracking(ctx context.Context, workspaceID string) error {
	if workspaceID != "" {
		return p.client.StopTimeTracking(ctx, workspaceID)
	}

	teams, err := p.client.getTeams(ctx)
	if err != nil {
		return err
	}
	if len(teams.Teams) == 0 {
		return fmt.Errorf("no ClickUp teams found")
	}

	var lastErr error
	for _, team := range teams.Teams {
		// We try to stop on all teams. Usually only one will have a running timer.
		// StopTimeTracking on a team with no running timer might return an error or be a no-op.
		// We'll ignore errors for teams that might not have a timer.
		if err := p.client.StopTimeTracking(ctx, team.ID.String()); err != nil {
			lastErr = err
		}
	}
	// If we have only one team and it failed, return that error.
	if len(teams.Teams) == 1 {
		return lastErr
	}
	return nil
}

func (p *Provider) GetRunningTimeEntry(ctx context.Context, workspaceID string) (*provider.TimeEntry, error) {
	if workspaceID != "" {
		dto, err := p.client.GetRunningTimeEntry(ctx, workspaceID)
		if err != nil {
			return nil, err
		}
		if dto == nil {
			return nil, nil
		}
		entry := p.mapTimeEntry(*dto)
		entry.IsRunning = true
		return &entry, nil
	}

	teams, err := p.client.getTeams(ctx)
	if err != nil {
		return nil, err
	}

	for _, team := range teams.Teams {
		dto, err := p.client.GetRunningTimeEntry(ctx, team.ID.String())
		if err != nil {
			continue // Try next team
		}
		if dto != nil {
			entry := p.mapTimeEntry(*dto)
			entry.IsRunning = true
			return &entry, nil
		}
	}

	return nil, nil
}

func (p *Provider) GetTimeEntries(ctx context.Context, workspaceID string, taskID string) ([]provider.TimeEntry, error) {
	if workspaceID == "" {
		teams, err := p.client.getTeams(ctx)
		if err != nil {
			return nil, err
		}
		if len(teams.Teams) == 0 {
			return nil, fmt.Errorf("no ClickUp teams found")
		}
		workspaceID = teams.Teams[0].ID.String()
	}
	dtos, err := p.client.GetTimeEntries(ctx, workspaceID, taskID)
	if err != nil {
		return nil, err
	}
	out := make([]provider.TimeEntry, 0, len(dtos))
	for _, dto := range dtos {
		out = append(out, p.mapTimeEntry(dto))
	}
	return out, nil
}

func (p *Provider) CreateTimeEntry(ctx context.Context, workspaceID string, taskID string, entry provider.TimeEntry) (provider.TimeEntry, error) {
	if workspaceID == "" {
		teams, err := p.client.getTeams(ctx)
		if err != nil {
			return provider.TimeEntry{}, err
		}
		if len(teams.Teams) == 0 {
			return provider.TimeEntry{}, fmt.Errorf("no ClickUp teams found")
		}
		workspaceID = teams.Teams[0].ID.String()
	}

	req := CreateTimeEntryRequest{
		TID:         taskID,
		Start:       entry.StartUnixMS,
		Description: entry.Description,
	}
	if entry.EndUnixMS != nil {
		req.End = *entry.EndUnixMS
	}
	if entry.DurationMS > 0 {
		req.Duration = entry.DurationMS
	}

	if err := p.client.CreateTimeEntry(ctx, workspaceID, req); err != nil {
		return provider.TimeEntry{}, err
	}
	// ClickUp Create Time Entry might not return the created entry, so we might want to fetch it.
	// For now return the input.
	return entry, nil
}

func (p *Provider) UpdateTimeEntry(ctx context.Context, workspaceID string, entryID string, update provider.TimeEntryUpdate) (provider.TimeEntry, error) {
	req := UpdateTimeEntryRequest{
		Description: update.Description,
		Start:       update.StartUnixMS,
		End:         update.EndUnixMS,
	}

	if workspaceID != "" {
		if err := p.client.UpdateTimeEntry(ctx, workspaceID, entryID, req); err != nil {
			return provider.TimeEntry{}, err
		}
		return provider.TimeEntry{ID: entryID}, nil
	}

	teams, err := p.client.getTeams(ctx)
	if err != nil {
		return provider.TimeEntry{}, err
	}

	var lastErr error
	for _, team := range teams.Teams {
		if err := p.client.UpdateTimeEntry(ctx, team.ID.String(), entryID, req); err == nil {
			return provider.TimeEntry{ID: entryID}, nil
		} else {
			lastErr = err
		}
	}

	if lastErr != nil {
		return provider.TimeEntry{}, lastErr
	}
	return provider.TimeEntry{}, fmt.Errorf("failed to update time entry: workspace not found")
}

func (p *Provider) DeleteTimeEntry(ctx context.Context, workspaceID string, entryID string) error {
	if workspaceID != "" {
		return p.client.DeleteTimeEntry(ctx, workspaceID, entryID)
	}

	teams, err := p.client.getTeams(ctx)
	if err != nil {
		return err
	}

	var lastErr error
	for _, team := range teams.Teams {
		if err := p.client.DeleteTimeEntry(ctx, team.ID.String(), entryID); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}

	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("failed to delete time entry: workspace not found")
}

func (p *Provider) mapTimeEntry(dto TimeEntryDTO) provider.TimeEntry {
	start := parseUnixOrZero(dto.Start.String())
	endVal := parseUnixOrZero(dto.End.String())
	var end *int64
	if endVal > 0 {
		end = &endVal
	}
	duration := parseUnixOrZero(dto.Duration.String())
	
	taskID := ""
	taskTitle := ""
	if dto.Task != nil {
		taskID = dto.Task.ID.String()
		taskTitle = dto.Task.Name
	}

	return provider.TimeEntry{
		ID:          dto.ID,
		TaskID:      taskID,
		TaskTitle:   taskTitle,
		Description: dto.Description,
		StartUnixMS: start,
		EndUnixMS:   end,
		DurationMS:  duration,
		User: provider.User{
			ID:       dto.User.ID.String(),
			Provider: "clickup",
			Username: dto.User.Username,
			Email:    dto.User.Email,
		},
	}
}

func (p *Provider) mapTask(t TaskDTO, listID string) provider.Task {
	task := provider.Task{
		ID:            t.ID.String(),
		Provider:      "clickup",
		ExternalID:    t.ID.String(),
		ListID:        listID,
		Title:         t.Name,
		DescriptionMD: maybeDecodeRichText(t.Description),
		Status:        t.Status.Status,
		StatusColor:   t.Status.Color,
		CustomFields:  map[string]any{},
	}
	task.ListIDs = append(task.ListIDs, t.List.ID.String())
	for _, l := range t.Lists {
		task.ListIDs = append(task.ListIDs, l.ID.String())
	}
	if t.DueDate != nil {
		if parsedDue, err := strconv.ParseInt(t.DueDate.String(), 10, 64); err == nil {
			task.DueAtUnixMS = &parsedDue
		}
	}
	if t.TimeEstimate != nil {
		task.EstimateMS = t.TimeEstimate
	}
	if t.TimeSpent != nil {
		task.TimeTrackedMS = t.TimeSpent
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
			ID:           att.ID.String(),
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
	return task
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
				rendered := s
				if attributes, ok := part["attributes"].(map[string]any); ok {
					if code, ok := attributes["code"].(bool); ok && code {
						rendered = "`" + rendered + "`"
					}
					if link, ok := attributes["link"].(string); ok {
						rendered = fmt.Sprintf("[%s](%s)", rendered, link)
					}
				}
				buf.WriteString(rendered)
				continue
			}
			if m, ok := insert.(map[string]any); ok {
				content = m
			}
		}

		if text, ok := content["text"].(string); ok && text != "" {
			rendered := text
			if attributes, ok := part["attributes"].(map[string]any); ok {
				if code, ok := attributes["code"].(bool); ok && code {
					rendered = "`" + rendered + "`"
				}
				if link, ok := attributes["link"].(string); ok {
					rendered = fmt.Sprintf("[%s](%s)", rendered, link)
				}
			}
			buf.WriteString(rendered)
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

		// Handle Embeds, Attachments & Bookmarks more generically
		url := ""
		for _, key := range []string{"url", "link", "original_url"} {
			if s, ok := content[key].(string); ok && s != "" {
				url = s
				break
			}
		}

		// Check nested objects (embed, attachment, bookmark, link_preview)
		for _, nestedKey := range []string{"embed", "attachment", "link_preview", "bookmark"} {
			if nested, ok := content[nestedKey].(map[string]any); ok {
				for _, key := range []string{"url", "link", "original_url"} {
					if s, ok := nested[key].(string); ok && s != "" {
						url = s
						break
					}
				}
			}
			if url != "" {
				break
			}
		}

		if url != "" {
			buf.WriteString(fmt.Sprintf("[Link: %s]", url))
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
