-- +goose Up
ALTER TABLE workspaces ADD COLUMN default_runtime TEXT NULL;

-- +goose Down
ALTER TABLE workspaces DROP COLUMN default_runtime;
