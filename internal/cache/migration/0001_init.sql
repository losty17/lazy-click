CREATE TABLE IF NOT EXISTS spaces (
    id TEXT PRIMARY KEY,
    provider TEXT NOT NULL,
    external_id TEXT NOT NULL,
    workspace_id TEXT,
    name TEXT NOT NULL,
    updated_at DATETIME
);

CREATE TABLE IF NOT EXISTS lists (
    id TEXT PRIMARY KEY,
    provider TEXT NOT NULL,
    external_id TEXT NOT NULL,
    space_id TEXT NOT NULL,
    name TEXT NOT NULL,
    updated_at DATETIME
);

CREATE TABLE IF NOT EXISTS tasks (
    id TEXT PRIMARY KEY,
    provider TEXT NOT NULL,
    external_id TEXT NOT NULL,
    list_id TEXT NOT NULL,
    title TEXT NOT NULL,
    description_md TEXT,
    status TEXT NOT NULL,
    priority_key TEXT,
    priority_label TEXT,
    priority_rank INTEGER DEFAULT 0,
    due_at_unix_ms INTEGER,
    custom_fields_json TEXT,
    updated_at_unix INTEGER NOT NULL,
    updated_at DATETIME
);

CREATE TABLE IF NOT EXISTS tags (
    id TEXT PRIMARY KEY,
    provider TEXT NOT NULL,
    external_id TEXT NOT NULL,
    name TEXT NOT NULL,
    color TEXT,
    updated_at DATETIME
);

CREATE TABLE IF NOT EXISTS task_tags (
    task_id TEXT NOT NULL,
    tag_id TEXT NOT NULL,
    PRIMARY KEY (task_id, tag_id)
);

CREATE TABLE IF NOT EXISTS comments (
    id TEXT PRIMARY KEY,
    task_id TEXT NOT NULL,
    author_id TEXT,
    author_name TEXT,
    body_md TEXT NOT NULL,
    created_at_unix INTEGER NOT NULL,
    updated_at DATETIME
);

CREATE TABLE IF NOT EXISTS sync_queue (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    provider TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    entity_id TEXT NOT NULL,
    operation TEXT NOT NULL,
    payload_json TEXT NOT NULL,
    state TEXT NOT NULL DEFAULT 'pending',
    last_error TEXT,
    retry_count INTEGER NOT NULL DEFAULT 0,
    created_at_unix INTEGER NOT NULL,
    updated_at_unix INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_lists_space_id ON lists(space_id);
CREATE INDEX IF NOT EXISTS idx_tasks_list_id ON tasks(list_id);
CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
CREATE INDEX IF NOT EXISTS idx_tasks_updated ON tasks(updated_at_unix);
CREATE INDEX IF NOT EXISTS idx_comments_task_id ON comments(task_id);
CREATE INDEX IF NOT EXISTS idx_sync_queue_state_created ON sync_queue(state, created_at_unix);
