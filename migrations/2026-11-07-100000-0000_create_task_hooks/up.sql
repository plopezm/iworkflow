CREATE TABLE task_hooks (
    id TEXT NOT NULL PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    phase TEXT NOT NULL CHECK (phase IN ('pre', 'post')),
    event TEXT NOT NULL CHECK (event IN ('task_start', 'task_finish')),
    kind TEXT NOT NULL CHECK (kind IN ('script', 'agent')),
    position INTEGER NOT NULL,
    enabled INTEGER NOT NULL DEFAULT 1,
    on_failure TEXT NOT NULL CHECK (on_failure IN ('block', 'warn', 'ignore')),
    timeout_seconds INTEGER NOT NULL CHECK (timeout_seconds > 0),
    command TEXT NOT NULL DEFAULT '',
    args TEXT NOT NULL DEFAULT '[]',
    env TEXT NOT NULL DEFAULT '[]',
    use_shell INTEGER NOT NULL DEFAULT 0,
    prompt TEXT NOT NULL DEFAULT '',
    runtime TEXT NOT NULL DEFAULT '',
    model TEXT NOT NULL DEFAULT '',
    thinking TEXT NOT NULL DEFAULT '',
    profile TEXT NOT NULL DEFAULT ''
);

CREATE INDEX idx_task_hooks_workspace ON task_hooks (workspace_id, event, phase, position);

CREATE TABLE task_hook_skips (
    task_id TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    hook_id TEXT NOT NULL REFERENCES task_hooks(id) ON DELETE CASCADE,
    PRIMARY KEY (task_id, hook_id)
);
