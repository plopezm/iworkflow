-- +goose Up
-- Dependency edges between tasks of one workspace. `trigger_event` names the event of the
-- predecessor that starts the successor; edge conditions will extend this table later.
CREATE TABLE task_dependencies (
    predecessor_id TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    successor_id TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    trigger_event TEXT NOT NULL DEFAULT 'merged',
    created_at TIMESTAMP NOT NULL,
    PRIMARY KEY (predecessor_id, successor_id)
);

CREATE INDEX task_dependencies_workspace_id_idx ON task_dependencies (workspace_id);
CREATE INDEX task_dependencies_successor_id_idx ON task_dependencies (successor_id);

-- Node coordinates of a task on its workspace board canvas.
CREATE TABLE task_positions (
    task_id TEXT PRIMARY KEY NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    x DOUBLE NOT NULL,
    y DOUBLE NOT NULL
);

-- +goose Down
DROP TABLE task_positions;
DROP INDEX task_dependencies_successor_id_idx;
DROP INDEX task_dependencies_workspace_id_idx;
DROP TABLE task_dependencies;
