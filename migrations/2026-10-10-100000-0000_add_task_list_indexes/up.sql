DROP INDEX tasks_workspace_id_idx;
DROP INDEX tasks_status_idx;
CREATE INDEX tasks_status_created_at_idx ON tasks (status, created_at);
CREATE INDEX tasks_workspace_created_at_idx ON tasks (workspace_id, created_at);
