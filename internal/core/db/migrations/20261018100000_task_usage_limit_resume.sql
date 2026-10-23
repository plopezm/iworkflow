-- +goose Up
ALTER TABLE tasks ADD COLUMN resume_at TIMESTAMP NULL;
ALTER TABLE tasks ADD COLUMN resume_attempts INTEGER NOT NULL DEFAULT 0;

-- +goose Down
UPDATE tasks SET status = 'interrupted' WHERE status = 'rate_limited';
ALTER TABLE tasks DROP COLUMN resume_attempts;
ALTER TABLE tasks DROP COLUMN resume_at;
