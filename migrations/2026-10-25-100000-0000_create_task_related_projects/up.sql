CREATE TABLE task_related_projects (
    task_id TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    position INTEGER NOT NULL,
    PRIMARY KEY (task_id, workspace_id)
);

CREATE INDEX task_related_projects_workspace_id_idx ON task_related_projects (workspace_id);
