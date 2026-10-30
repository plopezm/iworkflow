-- +goose Up
ALTER TABLE tasks ADD COLUMN priority TEXT NOT NULL DEFAULT 'normal';

-- +goose Down
ALTER TABLE tasks DROP COLUMN priority;
