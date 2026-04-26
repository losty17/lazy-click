package cache

import (
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *Repository {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	err = db.AutoMigrate(
		&TaskEntity{},
		&ListEntity{},
		&SpaceEntity{},
		&SyncQueueEntity{},
		&TaskListJoinEntity{},
		&CommentEntity{},
		&AttachmentEntity{},
		&TagEntity{},
		&TaskTagEntity{},
	)
	if err != nil {
		t.Fatalf("failed to migrate database: %v", err)
	}

	return NewRepository(db)
}

func TestRemapEntityID_UpdatesPayload(t *testing.T) {
	repo := setupTestDB(t)

	oldID := "tmp_task_123"
	newID := "real_task_456"

	// 1. Create a task with old ID
	task := TaskEntity{
		ID:        oldID,
		Provider:  "clickup",
		Title:     "Test Task",
		SyncState: SyncStatePendingCreate,
	}
	if err := repo.SaveTasks([]TaskEntity{task}); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// 2. Queue an update with old ID in payload
	payload := fmt.Sprintf(`{"task_id":"%s","update":{"name":"Updated Name"}}`, oldID)
	err := repo.EnqueueSync(SyncQueueEntity{
		Provider:    "clickup",
		EntityType:  "task",
		EntityID:    oldID,
		Operation:   "update_task",
		PayloadJSON: payload,
		State:       "pending",
	})
	if err != nil {
		t.Fatalf("failed to enqueue sync: %v", err)
	}

	// 3. Remap ID
	if err := repo.RemapEntityID(oldID, newID, "task"); err != nil {
		t.Fatalf("failed to remap entity ID: %v", err)
	}

	// 4. Verify task ID is updated
	var updatedTask TaskEntity
	if err := repo.db.First(&updatedTask, "id = ?", newID).Error; err != nil {
		t.Errorf("task not found with new ID: %v", err)
	}

	// 5. Verify sync queue payload is updated
	var syncItem SyncQueueEntity
	if err := repo.db.First(&syncItem, "entity_id = ?", newID).Error; err != nil {
		t.Errorf("sync item not found with new entity ID: %v", err)
	}

	expectedPayload := fmt.Sprintf(`{"task_id":"%s","update":{"name":"Updated Name"}}`, newID)
	if syncItem.PayloadJSON != expectedPayload {
		t.Errorf("payload not updated. Got %s, want %s", syncItem.PayloadJSON, expectedPayload)
	}
}

func TestRemapEntityID_UpdatesListReferences(t *testing.T) {
	repo := setupTestDB(t)

	oldListID := "tmp_list_1"
	newListID := "real_list_1"

	// Queue a task creation in the temporary list
	payload := fmt.Sprintf(`{"list_id":"%s","task":{"title":"New Task"}}`, oldListID)
	err := repo.EnqueueSync(SyncQueueEntity{
		Provider:    "clickup",
		EntityType:  "task",
		EntityID:    "tmp_task_1",
		Operation:   "create_task",
		PayloadJSON: payload,
		State:       "pending",
	})
	if err != nil {
		t.Fatalf("failed to enqueue sync: %v", err)
	}

	// Remap list ID
	if err := repo.RemapEntityID(oldListID, newListID, "list"); err != nil {
		t.Fatalf("failed to remap list ID: %v", err)
	}

	// Verify sync queue payload is updated
	var syncItem SyncQueueEntity
	if err := repo.db.First(&syncItem, "entity_id = ?", "tmp_task_1").Error; err != nil {
		t.Fatalf("sync item not found")
	}

	expectedPayload := fmt.Sprintf(`{"list_id":"%s","task":{"title":"New Task"}}`, newListID)
	if syncItem.PayloadJSON != expectedPayload {
		t.Errorf("payload not updated. Got %s, want %s", syncItem.PayloadJSON, expectedPayload)
	}
}

func TestGetTasksByQuery_ExcludesPendingDelete(t *testing.T) {
	repo := setupTestDB(t)

	tasks := []TaskEntity{
		{ID: "task1", Title: "Task 1", SyncState: SyncStateSynced, Provider: "p1", ListID: "l1"},
		{ID: "task2", Title: "Task 2", SyncState: SyncStatePendingDelete, Provider: "p1", ListID: "l1"},
	}
	if err := repo.SaveTasks(tasks); err != nil {
		t.Fatalf("failed to save tasks: %v", err)
	}

	q := TaskListQuery{Provider: "p1", ListID: "l1"}
	found, err := repo.GetTasksByQuery(q)
	if err != nil {
		t.Fatalf("GetTasksByQuery failed: %v", err)
	}

	if len(found) != 1 {
		t.Errorf("expected 1 task, got %d", len(found))
	}
	for _, task := range found {
		if task.ID == "task2" {
			t.Errorf("found task marked for deletion: %s", task.ID)
		}
	}
}

func TestGetListsByQuery_ExcludesPendingDelete(t *testing.T) {
	repo := setupTestDB(t)

	lists := []ListEntity{
		{ID: "list1", Name: "List 1", SyncState: SyncStateSynced, Provider: "p1"},
		{ID: "list2", Name: "List 2", SyncState: SyncStatePendingDelete, Provider: "p1"},
	}
	for _, l := range lists {
		if err := repo.db.Create(&l).Error; err != nil {
			t.Fatalf("failed to create list: %v", err)
		}
	}

	q := ListQuery{Provider: "p1"}
	found, err := repo.GetListsByQuery(q)
	if err != nil {
		t.Fatalf("GetListsByQuery failed: %v", err)
	}

	if len(found) != 1 {
		t.Errorf("expected 1 list, got %d", len(found))
	}
	for _, list := range found {
		if list.ID == "list2" {
			t.Errorf("found list marked for deletion: %s", list.ID)
		}
	}
}

