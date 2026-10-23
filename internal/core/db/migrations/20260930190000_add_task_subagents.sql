-- +goose Up
ALTER TABLE tasks ADD COLUMN subagents_active INTEGER NOT NULL DEFAULT 0;
ALTER TABLE tasks ADD COLUMN subagents_waiting INTEGER NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE tasks DROP COLUMN subagents_waiting;
ALTER TABLE tasks DROP COLUMN subagents_active;
