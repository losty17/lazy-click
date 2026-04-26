package provider

import "context"

type ProjectProvider interface {
	GetCurrentUser(ctx context.Context) (User, error)
	GetSpaces(ctx context.Context) ([]Space, error)
	GetLists(ctx context.Context, spaceID string) ([]TaskList, error)
	GetTasks(ctx context.Context, listID string, filter TaskFilter) ([]Task, error)
	GetTask(ctx context.Context, taskID string) (Task, error)
	GetTaskComments(ctx context.Context, taskID string) ([]Comment, error)

	// Task CRUD
	CreateTask(ctx context.Context, listID string, task Task) (Task, error)
	UpdateTask(ctx context.Context, taskID string, data TaskUpdate) (Task, error)
	DeleteTask(ctx context.Context, taskID string) error

	// List CRUD
	CreateList(ctx context.Context, spaceID string, name string) (TaskList, error)
	UpdateList(ctx context.Context, listID string, name string) (TaskList, error)
	DeleteList(ctx context.Context, listID string) error

	// Comment CRUD
	AddComment(ctx context.Context, taskID string, text string) (Comment, error)
	UpdateComment(ctx context.Context, commentID string, text string) (Comment, error)
	DeleteComment(ctx context.Context, commentID string) error
}
