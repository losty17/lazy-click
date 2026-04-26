package clickup

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"lazy-click/internal/provider"
)

const baseURL = "https://api.clickup.com/api/v2"

type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

func NewClient(token string) *Client {
	return &Client{
		baseURL:    baseURL,
		token:      token,
		httpClient: http.DefaultClient,
	}
}

func (c *Client) GetSpaces(ctx context.Context) (*GetSpacesResponse, error) {
	// ClickUp spaces are nested under teams.
	teamsResp, err := c.getTeams(ctx)
	if err != nil {
		return nil, err
	}

	out := &GetSpacesResponse{}
	for _, team := range teamsResp.Teams {
		teamSpaces, err := c.getSpacesByTeam(ctx, team.ID.String())
		if err != nil {
			return nil, err
		}
		for i := range teamSpaces.Spaces {
			teamSpaces.Spaces[i].TeamID = team.ID.String()
			teamSpaces.Spaces[i].TeamName = team.Name
		}
		out.Spaces = append(out.Spaces, teamSpaces.Spaces...)
	}
	return out, nil
}

func (c *Client) GetLists(ctx context.Context, spaceID string) (*GetListsResponse, error) {
	spaceLists, err := c.getListsBySpace(ctx, spaceID)
	if err != nil {
		return nil, err
	}

	folders, err := c.getFoldersBySpace(ctx, spaceID)
	if err != nil {
		return nil, err
	}

	out := &GetListsResponse{
		Lists: make([]ListDTO, 0, len(spaceLists.Lists)),
	}
	out.Lists = append(out.Lists, spaceLists.Lists...)

	seen := make(map[string]struct{}, len(out.Lists))
	for _, list := range out.Lists {
		seen[list.ID.String()] = struct{}{}
	}

	for _, folder := range folders.Folders {
		folderLists, err := c.getListsByFolder(ctx, folder.ID.String())
		if err != nil {
			return nil, err
		}
		for _, list := range folderLists.Lists {
			if _, exists := seen[list.ID.String()]; exists {
				continue
			}
			seen[list.ID.String()] = struct{}{}
			out.Lists = append(out.Lists, list)
		}
	}

	return out, nil
}

func (c *Client) GetTasks(ctx context.Context, listID string, filter provider.TaskFilter) (*GetTasksResponse, error) {
	all := &GetTasksResponse{Tasks: []TaskDTO{}}
	page := 0
	for {
		resp, err := c.getTasksPage(ctx, listID, filter, page)
		if err != nil {
			return nil, err
		}
		all.Tasks = append(all.Tasks, resp.Tasks...)
		if resp.LastPage {
			all.LastPage = true
			break
		}
		page++
		if page > 200 {
			return nil, fmt.Errorf("clickup task pagination exceeded safety limit for list %s", listID)
		}
	}
	return all, nil
}

func (c *Client) getTasksPage(ctx context.Context, listID string, filter provider.TaskFilter, page int) (*GetTasksResponse, error) {
	values := url.Values{}
	values.Set("include_closed", strconv.FormatBool(filter.IncludeClosed))
	values.Set("subtasks", "true")
	values.Set("page", strconv.Itoa(page))
	if len(filter.Statuses) > 0 {
		for _, status := range filter.Statuses {
			values.Add("statuses[]", status)
		}
	}
	if len(filter.AssigneeIDs) > 0 {
		for _, id := range filter.AssigneeIDs {
			values.Add("assignees[]", id)
		}
	}

	path := "/list/" + listID + "/task"
	if encoded := values.Encode(); encoded != "" {
		path += "?" + encoded
	}

	var resp GetTasksResponse
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) GetCurrentUser(ctx context.Context) (*UserDTO, error) {
	var resp struct {
		User UserDTO `json:"user"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/user", nil, &resp); err != nil {
		return nil, err
	}
	return &resp.User, nil
}

func (c *Client) GetTask(ctx context.Context, taskID string) (*TaskDTO, error) {
	var resp TaskDTO
	if err := c.doJSON(ctx, http.MethodGet, "/task/"+taskID, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) GetTaskComments(ctx context.Context, taskID string) (*GetTaskCommentsResponse, error) {
	var resp GetTaskCommentsResponse
	if err := c.doJSON(ctx, http.MethodGet, "/task/"+taskID+"/comment", nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) UpdateTask(ctx context.Context, taskID string, req UpdateTaskRequest) error {
	return c.doJSON(ctx, http.MethodPut, "/task/"+taskID, req, nil)
}

func (c *Client) CreateTask(ctx context.Context, listID string, req CreateTaskRequest) (*TaskDTO, error) {
	var resp TaskDTO
	if err := c.doJSON(ctx, http.MethodPost, "/list/"+listID+"/task", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) DeleteTask(ctx context.Context, taskID string) error {
	return c.doJSON(ctx, http.MethodDelete, "/task/"+taskID, nil, nil)
}

func (c *Client) CreateList(ctx context.Context, spaceID string, req CreateListRequest) (*ListDTO, error) {
	var resp ListDTO
	if err := c.doJSON(ctx, http.MethodPost, "/space/"+spaceID+"/list", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) UpdateList(ctx context.Context, listID string, req UpdateListRequest) (*ListDTO, error) {
	var resp ListDTO
	if err := c.doJSON(ctx, http.MethodPut, "/list/"+listID, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) DeleteList(ctx context.Context, listID string) error {
	return c.doJSON(ctx, http.MethodDelete, "/list/"+listID, nil, nil)
}

func (c *Client) AddComment(ctx context.Context, taskID string, text string) (*AddCommentResponse, error) {
	body := AddCommentRequest{CommentText: text, NotifyAll: true}
	var resp AddCommentResponse
	if err := c.doJSON(ctx, http.MethodPost, "/task/"+taskID+"/comment", body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) UpdateComment(ctx context.Context, commentID string, text string) error {
	body := AddCommentRequest{CommentText: text}
	return c.doJSON(ctx, http.MethodPut, "/comment/"+commentID, body, nil)
}

func (c *Client) DeleteComment(ctx context.Context, commentID string) error {
	return c.doJSON(ctx, http.MethodDelete, "/comment/"+commentID, nil, nil)
}

func (c *Client) getTeams(ctx context.Context) (*GetTeamsResponse, error) {
	var resp GetTeamsResponse
	if err := c.doJSON(ctx, http.MethodGet, "/team", nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) getSpacesByTeam(ctx context.Context, teamID string) (*GetSpacesResponse, error) {
	var resp GetSpacesResponse
	if err := c.doJSON(ctx, http.MethodGet, "/team/"+teamID+"/space", nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) getFoldersBySpace(ctx context.Context, spaceID string) (*GetFoldersResponse, error) {
	values := url.Values{}
	values.Set("archived", "false")
	path := "/space/" + spaceID + "/folder?" + values.Encode()

	var resp GetFoldersResponse
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) getListsBySpace(ctx context.Context, spaceID string) (*GetListsResponse, error) {
	values := url.Values{}
	values.Set("archived", "false")
	path := "/space/" + spaceID + "/list?" + values.Encode()

	var resp GetListsResponse
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) getListsByFolder(ctx context.Context, folderID string) (*GetListsResponse, error) {
	values := url.Values{}
	values.Set("archived", "false")
	path := "/folder/" + folderID + "/list?" + values.Encode()

	var resp GetListsResponse
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) doJSON(ctx context.Context, method string, path string, in any, out any) error {
	var body io.Reader
	if in != nil {
		payload, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", c.token)
	req.Header.Set("Content-Type", "application/json")

	res, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	if res.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("clickup api %s %s failed: %d %s", method, path, res.StatusCode, strings.TrimSpace(string(raw)))
	}
	if out == nil || len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, out)
}
