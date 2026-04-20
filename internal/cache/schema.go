package cache

import "time"

type SpaceEntity struct {
	ID          string `gorm:"primaryKey;size:128"`
	Provider    string `gorm:"index;size:32;not null"`
	ExternalID  string `gorm:"index;size:128;not null"`
	WorkspaceID string `gorm:"index;size:128"`
	Name        string `gorm:"size:256;not null"`
	UpdatedAt   time.Time
}

type ListEntity struct {
	ID             string `gorm:"primaryKey;size:128"`
	Provider       string `gorm:"index;size:32;not null"`
	ExternalID     string `gorm:"index;size:128;not null"`
	SpaceID        string `gorm:"index;size:128;not null"`
	Name           string `gorm:"size:256;not null"`
	Favorite       bool   `gorm:"index;not null;default:false"`
	LastOpenedUnix int64  `gorm:"index;not null;default:0"`
	LastSyncedUnix int64  `gorm:"index;not null;default:0"`
	UpdatedAt      time.Time
}

type TaskEntity struct {
	ID               string `gorm:"primaryKey;size:128"`
	Provider         string `gorm:"index;size:32;not null"`
	ExternalID       string `gorm:"index;size:128;not null"`
	ListID           string `gorm:"index;size:128;not null"`
	ParentTaskID     string `gorm:"index;size:128"`
	IsSubtask        bool   `gorm:"index;not null;default:false"`
	Title            string `gorm:"size:1024;not null"`
	DescriptionMD    string `gorm:"type:text"`
	Status           string `gorm:"index;size:64;not null"`
	StatusColor      string `gorm:"size:16"`
	PriorityKey      string `gorm:"size:32"`
	PriorityLabel    string `gorm:"size:64"`
	PriorityRank     int
	PriorityColor    string `gorm:"size:16"`
	EstimateMS       *int64
	DueAtUnixMS      *int64
	AssigneesJSON    string `gorm:"type:text"`
	CustomFieldsJSON string `gorm:"type:text"`
	UpdatedAtUnix    int64  `gorm:"index"`
	LastFetchedUnix  int64  `gorm:"index;not null;default:0"`
	UpdatedAt        time.Time
}

type AppStateEntity struct {
	Key       string `gorm:"primaryKey;size:128"`
	Value     string `gorm:"type:text"`
	UpdatedAt time.Time
}

type TagEntity struct {
	ID         string `gorm:"primaryKey;size:128"`
	Provider   string `gorm:"index;size:32;not null"`
	ExternalID string `gorm:"index;size:128;not null"`
	Name       string `gorm:"size:128;not null"`
	Color      string `gorm:"size:16"`
	UpdatedAt  time.Time
}

type TaskTagEntity struct {
	TaskID string `gorm:"primaryKey;size:128"`
	TagID  string `gorm:"primaryKey;size:128"`
}

type CommentEntity struct {
	ID            string `gorm:"primaryKey;size:128"`
	TaskID        string `gorm:"index;size:128;not null"`
	AuthorID      string `gorm:"size:128"`
	AuthorName    string `gorm:"size:128"`
	BodyMD        string `gorm:"type:text;not null"`
	CreatedAtUnix int64  `gorm:"index"`
	UpdatedAt     time.Time
}

type SyncQueueEntity struct {
	ID            uint64 `gorm:"primaryKey;autoIncrement"`
	Provider      string `gorm:"index;size:32;not null"`
	EntityType    string `gorm:"index;size:32;not null"`
	EntityID      string `gorm:"index;size:128;not null"`
	Operation     string `gorm:"size:32;not null"`
	PayloadJSON   string `gorm:"type:text;not null"`
	State         string `gorm:"index;size:32;not null;default:pending"`
	LastError     string `gorm:"type:text"`
	RetryCount    int    `gorm:"not null;default:0"`
	CreatedAtUnix int64  `gorm:"index;not null"`
	UpdatedAtUnix int64  `gorm:"not null"`
}
