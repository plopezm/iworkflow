DROP INDEX tasks_workspace_created_at_idx;
DROP INDEX tasks_status_created_at_idx;
CREATE INDEX tasks_workspace_id_idx ON tasks (workspace_id);
CREATE INDEX tasks_status_idx ON tasks (status);
