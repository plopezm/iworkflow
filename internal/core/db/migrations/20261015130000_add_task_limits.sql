-- +goose Up
ALTER TABLE tasks ADD COLUMN memory_limit_mb INTEGER NULL;
ALTER TABLE tasks ADD COLUMN cpu_limit_percent INTEGER NULL;

-- +goose Down
ALTER TABLE tasks DROP COLUMN cpu_limit_percent;
ALTER TABLE tasks DROP COLUMN memory_limit_mb;
