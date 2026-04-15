package clickup

import "encoding/json"

type GetTeamsResponse struct {
	Teams []TeamDTO `json:"teams"`
}

type TeamDTO struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type GetSpacesResponse struct {
	Spaces []SpaceDTO `json:"spaces"`
}

type SpaceDTO struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type GetListsResponse struct {
	Lists []ListDTO `json:"lists"`
}

type GetFoldersResponse struct {
	Folders []FolderDTO `json:"folders"`
}

type FolderDTO struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ListDTO struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type GetTasksResponse struct {
	Tasks []TaskDTO `json:"tasks"`
}

type TaskDTO struct {
	ID           string           `json:"id"`
	Name         string           `json:"name"`
	Description  string           `json:"description"`
	Status       TaskStatusDTO    `json:"status"`
	Priority     *TaskPriorityDTO `json:"priority"`
	DueDate      *string          `json:"due_date"`
	Tags         []TagDTO         `json:"tags"`
	CustomFields []CustomFieldDTO `json:"custom_fields"`
	List         ListDTO          `json:"list"`
}

type TaskStatusDTO struct {
	Status string `json:"status"`
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
	ID    string `json:"id"`
	Name  string `json:"name"`
	Type  string `json:"type"`
	Value any    `json:"value"`
}

type UpdateTaskRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	Status      *string `json:"status,omitempty"`
	DueDate     *int64  `json:"due_date,omitempty"`
}

type AddCommentRequest struct {
	CommentText string `json:"comment_text"`
	NotifyAll   bool   `json:"notify_all"`
}

type AddCommentResponse struct {
	ID      string          `json:"id"`
	Comment json.RawMessage `json:"comment"`
	Date    string          `json:"date"`
	User    UserDTO         `json:"user"`
}

type UserDTO struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
}
