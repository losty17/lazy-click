package clickup

import (
	"bytes"
	"encoding/json"
)

type ClickUpID string

func (id *ClickUpID) UnmarshalJSON(data []byte) error {
	var fs FlexString
	if err := fs.UnmarshalJSON(data); err != nil {
		return err
	}
	*id = ClickUpID(fs)
	return nil
}

func (id ClickUpID) String() string {
	return string(id)
}

type FlexString string

func (fs *FlexString) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		*fs = ""
		return nil
	}

	if trimmed[0] == '"' {
		var s string
		if err := json.Unmarshal(trimmed, &s); err != nil {
			return err
		}
		*fs = FlexString(s)
		return nil
	}

	var n json.Number
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	dec.UseNumber()
	if err := dec.Decode(&n); err != nil {
		return err
	}
	*fs = FlexString(n.String())
	return nil
}

func (fs FlexString) String() string {
	return string(fs)
}

type GetTeamsResponse struct {
	Teams []TeamDTO `json:"teams"`
}

type TeamDTO struct {
	ID   ClickUpID `json:"id"`
	Name string    `json:"name"`
}

type GetSpacesResponse struct {
	Spaces []SpaceDTO `json:"spaces"`
}

type SpaceDTO struct {
	ID       ClickUpID `json:"id"`
	Name     string    `json:"name"`
	TeamID   string    `json:"-"`
	TeamName string    `json:"-"`
}

type GetListsResponse struct {
	Lists []ListDTO `json:"lists"`
}

type GetFoldersResponse struct {
	Folders []FolderDTO `json:"folders"`
}

type FolderDTO struct {
	ID   ClickUpID `json:"id"`
	Name string    `json:"name"`
}

type ListDTO struct {
	ID   ClickUpID `json:"id"`
	Name string    `json:"name"`
}

type GetTasksResponse struct {
	Tasks    []TaskDTO `json:"tasks"`
	LastPage bool      `json:"last_page"`
}

type TaskDTO struct {
	ID           ClickUpID        `json:"id"`
	Name         string           `json:"name"`
	Description  string           `json:"description"`
	Status       TaskStatusDTO    `json:"status"`
	Priority     *TaskPriorityDTO `json:"priority"`
	TimeEstimate *int64           `json:"time_estimate"`
	TimeSpent    *int64           `json:"time_spent"`
	DueDate      *FlexString      `json:"due_date"`
	Parent       *string          `json:"parent"`
	Assignees    []UserDTO        `json:"assignees"`
	Tags         []TagDTO         `json:"tags"`
	Attachments  []AttachmentDTO  `json:"attachments"`
	CustomFields []CustomFieldDTO `json:"custom_fields"`
	List         ListDTO          `json:"list"`
	Lists        []ListDTO        `json:"lists"`
}

type AttachmentDTO struct {
	ID           ClickUpID  `json:"id"`
	Date         FlexString `json:"date"`
	Title        string     `json:"title"`
	Type         int        `json:"type"`
	Source       int       `json:"source"`
	Version      int       `json:"version"`
	Extension    string    `json:"extension"`
	ThumbnailSmall string  `json:"thumbnail_small"`
	ThumbnailLarge string  `json:"thumbnail_large"`
	IsStreamable bool      `json:"is_streamable"`
	URL          string    `json:"url"`
	EmailData    any       `json:"email_data"`
	Priority     int       `json:"priority"`
	Size         int64     `json:"size"`
}

type TaskStatusDTO struct {
	Status string `json:"status"`
	Color  string `json:"color"`
}

type TaskPriorityDTO struct {
	Priority   string `json:"priority"`
	Color      string `json:"color"`
	OrderIndex string `json:"orderindex"`
}

type TagDTO struct {
	Name  string `json:"name"`
	TagFg string `json:"tag_fg"`
}

type CustomFieldDTO struct {
	ID    ClickUpID `json:"id"`
	Name  string    `json:"name"`
	Type  string    `json:"type"`
	Value any       `json:"value"`
}

type UpdateTaskRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	Status      *string `json:"status,omitempty"`
	DueDate     *int64  `json:"due_date,omitempty"`
	Priority    *int    `json:"priority,omitempty"`
}

type CreateTaskRequest struct {
	Name        string  `json:"name"`
	Description string  `json:"description,omitempty"`
	Status      string  `json:"status,omitempty"`
	DueDate     *int64  `json:"due_date,omitempty"`
	Priority    *int    `json:"priority,omitempty"`
	Parent      *string `json:"parent,omitempty"`
}

type CreateListRequest struct {
	Name string `json:"name"`
}

type UpdateListRequest struct {
	Name string `json:"name"`
}

type AddCommentRequest struct {
	CommentText string `json:"comment_text"`
	NotifyAll   bool   `json:"notify_all"`
}

type AddCommentResponse struct {
	ID      ClickUpID       `json:"id"`
	Comment json.RawMessage `json:"comment"`
	Date    FlexString      `json:"date"`
	User    UserDTO         `json:"user"`
}

type GetTaskCommentsResponse struct {
	Comments []CommentDTO `json:"comments"`
}

type CommentDTO struct {
	ID      ClickUpID       `json:"id"`
	Comment json.RawMessage `json:"comment"`
	Date    FlexString      `json:"date"`
	User    UserDTO         `json:"user"`
}

type UserDTO struct {
	ID       ClickUpID `json:"id"`
	Username string    `json:"username"`
	Email    string    `json:"email"`
}
