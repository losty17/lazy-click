package provider

type User struct {
	ID       string
	Provider string
	Username string
	Email    string
}

type Priority struct {
	Key   string
	Label string
	Rank  int
	Color string
}

type Tag struct {
	ID    string
	Name  string
	Color string
}

type Task struct {
	ID            string
	Provider      string
	ExternalID    string
	ListID        string
	ParentTaskID  string
	IsSubtask     bool
	Title         string
	DescriptionMD string
	Status        string
	Priority      *Priority
	DueAtUnixMS   *int64
	Assignees     []User
	Tags          []Tag
	CustomFields  map[string]any
	UpdatedAtUnix int64
}

type Comment struct {
	ID            string
	TaskID        string
	Author        User
	BodyMD        string
	CreatedAtUnix int64
}

type Space struct {
	ID   string
	Name string
}

type TaskList struct {
	ID      string
	SpaceID string
	Name    string
}

type TaskFilter struct {
	Statuses      []string
	AssigneeIDs   []string
	Search        string
	IncludeClosed bool
}

type TaskUpdate struct {
	Title         *string
	DescriptionMD *string
	Status        *string
	DueAtUnixMS   *int64
	PriorityKey   *string
	TagIDs        []string
	CustomFields  map[string]any
}
