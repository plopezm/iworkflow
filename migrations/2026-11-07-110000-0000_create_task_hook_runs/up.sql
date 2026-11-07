CREATE TABLE task_hook_runs (
    id TEXT NOT NULL PRIMARY KEY,
    task_id TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    hook_id TEXT NOT NULL REFERENCES task_hooks(id) ON DELETE CASCADE,
    phase TEXT NOT NULL,
    event TEXT NOT NULL,
    status TEXT NOT NULL,
    exit_code INTEGER NOT NULL DEFAULT 0,
    output TEXT NOT NULL DEFAULT '',
    started_at TEXT NOT NULL,
    finished_at TEXT NOT NULL DEFAULT ''
);

CREATE INDEX idx_task_hook_runs_task ON task_hook_runs (task_id, started_at);
