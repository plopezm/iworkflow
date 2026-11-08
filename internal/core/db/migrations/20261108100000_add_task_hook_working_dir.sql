-- +goose Up
ALTER TABLE task_hooks ADD COLUMN working_dir TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE task_hooks DROP COLUMN working_dir;
