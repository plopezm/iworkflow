-- +goose Up
CREATE TABLE tasks (
    id TEXT PRIMARY KEY NOT NULL,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE RESTRICT,
    title TEXT NOT NULL,
    prompt TEXT NOT NULL,
    branch TEXT NOT NULL,
    base_branch TEXT NOT NULL,
    worktree_path TEXT NOT NULL,
    status TEXT NOT NULL,
    error TEXT,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL
);

CREATE INDEX tasks_workspace_id_idx ON tasks (workspace_id);
CREATE INDEX tasks_status_idx ON tasks (status);

CREATE TABLE task_events (
    seq INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
    task_id TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    payload TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL
);

CREATE INDEX task_events_task_id_seq_idx ON task_events (task_id, seq);

-- +goose Down
DROP TABLE task_events;
DROP TABLE tasks;
