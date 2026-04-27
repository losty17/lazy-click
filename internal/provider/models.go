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

type Attachment struct {
	ID           string
	Filename     string
	URL          string
	ThumbnailURL string
	Size         int64
	ContentType  string
}

type Task struct {
	ID            string
	Provider      string
	ExternalID    string
	ListID        string
	ListIDs       []string
	ParentTaskID  string
	IsSubtask     bool
	Title         string
	DescriptionMD string
	Status        string
	StatusColor   string
	Priority      *Priority
	EstimateMS    *int64
	TimeTrackedMS *int64
	DueAtUnixMS   *int64
	Assignees     []User
	Tags          []Tag
	Attachments   []Attachment
	CustomFields  map[string]any
	UpdatedAtUnix int64
}

type Comment struct {
	ID             string
	TaskID         string
	Author         User
	BodyMD         string
	RawPayloadJSON string
	CreatedAtUnix  int64
}

type Space struct {
	ID            string
	Name          string
	WorkspaceID   string
	WorkspaceName string
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

type TimeEntry struct {
	ID          string
	TaskID      string
	TaskTitle   string
	Description string
	StartUnixMS int64
	EndUnixMS   *int64
	DurationMS  int64
	IsRunning   bool
	User        User
}

type TimeEntryUpdate struct {
	Description *string
	StartUnixMS *int64
	EndUnixMS   *int64
}
