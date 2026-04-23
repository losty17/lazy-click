package provider

import "context"

type ProjectProvider interface {
	GetSpaces(ctx context.Context) ([]Space, error)
	GetLists(ctx context.Context, spaceID string) ([]TaskList, error)
	GetTasks(ctx context.Context, listID string, filter TaskFilter) ([]Task, error)
	GetTask(ctx context.Context, taskID string) (Task, error)
	GetTaskComments(ctx context.Context, taskID string) ([]Comment, error)
	UpdateTask(ctx context.Context, taskID string, data TaskUpdate) (Task, error)
	AddComment(ctx context.Context, taskID string, text string) (Comment, error)
}
