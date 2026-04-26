# Local-First Sync Engine Guide

This document describes the architectural approach for CRUD operations in `lazy-click`.

## Core Philosophy: Local-First
All user actions are applied to the local database **immediately**. The UI should reflect these changes instantly, giving a snappy feel even with high-latency APIs.

1.  **Action**: User edits a task.
2.  **Local Update**: The application updates the local SQLite cache and marks the record as `pending_update`.
3.  **Queue**: A sync item is added to the `sync_queue`.
4.  **Background Sync**: The `Engine` picks up the item and pushes it to the ClickUp API.
5.  **Reconciliation**: On success, the local record is marked as `synced`. On failure, it's marked as `error` with the reason.

## Entity Sync States
Every synced entity (Task, List, Comment) should have a `sync_state` column:

- `synced`: Matches the remote state.
- `pending_create`: Created locally, waiting for remote ID.
- `pending_update`: Edited locally, waiting for remote confirmation.
- `pending_delete`: Deleted locally, waiting for remote confirmation.
- `error`: Last sync attempt failed. Check `last_error` column.

## Handling Temporary IDs
When creating a new entity (e.g., a Task), we don't have a remote ID yet.
1.  Generate a local temporary ID (e.g., `tmp_task_uuid`).
2.  Save to local DB with `pending_create`.
3.  Enqueue `create_task` with the temporary ID.
4.  The Sync Engine calls the API, gets the real ID (e.g., `86abc123`).
5.  **Critical**: The Sync Engine must update the local DB:
    - Change the entity ID from `tmp_task_uuid` to `86abc123`.
    - Update all foreign keys in other tables (e.g., `comments.task_id`).
    - Update any other pending sync items in the queue that refer to the old temp ID.

## Permissions & Errors
Since ClickUp doesn't provide a granular "check-permission" endpoint:
- We assume the user has permission and proceed with the local update.
- If the API returns `403 Forbidden`, we transition the entity to the `error` state.
- The UI should display this error and offer a "Revert" or "Fix" option.

## Destructive Actions
To avoid unintended results:
1.  **Confirmation**: The TUI must require a hard confirmation (e.g., typing "DELETE" or a clear Y/N prompt) before calling the Sync Engine's delete method.
2.  **Soft-Delete**: Locally, we can "hide" the record immediately, but we keep it in the DB until the remote delete is confirmed.

## Single Field Changes
Updates should be granular. Instead of sending the whole task, the `UpdateTask` payload should only contain the changed fields. This reduces the risk of overwriting changes made elsewhere.

```go
type TaskUpdate struct {
    Status *string
    DueAt  *int64
    // ... other fields as pointers
}
```
Using pointers allows us to distinguish between "don't change" (nil) and "set to empty/null".
