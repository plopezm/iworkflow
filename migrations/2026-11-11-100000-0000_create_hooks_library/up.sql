-- The hooks move from one list per workspace to a library of hooks plus assignments (workspace, event, phase). The
-- hooks of the old shape are discarded on purpose, with their skips and run records.
DROP TABLE task_hook_skips;
DROP TABLE task_hook_runs;
DROP INDEX idx_task_hooks_workspace;
DROP TABLE task_hooks;

CREATE TABLE hooks (
    id TEXT NOT NULL PRIMARY KEY,
    name TEXT NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('script', 'agent')),
    timeout_seconds INTEGER NOT NULL CHECK (timeout_seconds > 0),
    command TEXT NOT NULL DEFAULT '',
    args TEXT NOT NULL DEFAULT '[]',
    env TEXT NOT NULL DEFAULT '[]',
    use_shell INTEGER NOT NULL DEFAULT 0,
    working_dir TEXT NOT NULL DEFAULT '',
    prompt TEXT NOT NULL DEFAULT '',
    runtime TEXT NOT NULL DEFAULT '',
    model TEXT NOT NULL DEFAULT '',
    thinking TEXT NOT NULL DEFAULT '',
    profile TEXT NOT NULL DEFAULT ''
);

CREATE TABLE hook_assignments (
    id TEXT NOT NULL PRIMARY KEY,
    hook_id TEXT NOT NULL REFERENCES hooks(id) ON DELETE CASCADE,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    event TEXT NOT NULL CHECK (event IN ('task_start', 'task_finish')),
    phase TEXT NOT NULL CHECK (phase IN ('pre', 'post')),
    position INTEGER NOT NULL,
    enabled INTEGER NOT NULL DEFAULT 1,
    on_failure TEXT NOT NULL CHECK (on_failure IN ('block', 'warn', 'ignore')),
    UNIQUE (hook_id, workspace_id, event, phase)
);

CREATE INDEX idx_hook_assignments_workspace ON hook_assignments (workspace_id, event, phase, position);

CREATE TABLE task_hook_skips (
    task_id TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    assignment_id TEXT NOT NULL REFERENCES hook_assignments(id) ON DELETE CASCADE,
    PRIMARY KEY (task_id, assignment_id)
);

-- A run keeps the name of its hook and its workspace instead of a foreign key, so the error history outlives the
-- hook or the assignment that produced it.
CREATE TABLE task_hook_runs (
    id TEXT NOT NULL PRIMARY KEY,
    task_id TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    hook_id TEXT NOT NULL,
    assignment_id TEXT NOT NULL,
    hook_name TEXT NOT NULL,
    workspace_id TEXT NOT NULL,
    phase TEXT NOT NULL,
    event TEXT NOT NULL,
    status TEXT NOT NULL,
    exit_code INTEGER NOT NULL DEFAULT 0,
    output TEXT NOT NULL DEFAULT '',
    started_at TEXT NOT NULL,
    finished_at TEXT NOT NULL DEFAULT ''
);

CREATE INDEX idx_task_hook_runs_task ON task_hook_runs (task_id, started_at);
CREATE INDEX idx_task_hook_runs_hook ON task_hook_runs (hook_id, started_at);
