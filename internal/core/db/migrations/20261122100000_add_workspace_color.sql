-- +goose Up
ALTER TABLE workspaces ADD COLUMN color TEXT NULL;

-- +goose Down
ALTER TABLE workspaces DROP COLUMN color;
