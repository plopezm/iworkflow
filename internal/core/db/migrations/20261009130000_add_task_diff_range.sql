-- +goose Up
ALTER TABLE tasks ADD COLUMN base_sha TEXT;
ALTER TABLE tasks ADD COLUMN head_sha TEXT;

-- +goose Down
ALTER TABLE tasks DROP COLUMN head_sha;
ALTER TABLE tasks DROP COLUMN base_sha;
