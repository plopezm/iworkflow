-- +goose Up
CREATE INDEX tasks_status_updated_at_idx ON tasks (status, updated_at);

-- +goose Down
DROP INDEX tasks_status_updated_at_idx;
