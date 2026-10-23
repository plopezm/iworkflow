-- +goose Up
ALTER TABLE agent_settings ADD COLUMN memory_limit_mb INTEGER NULL;
ALTER TABLE agent_settings ADD COLUMN cpu_limit_percent INTEGER NULL;

-- +goose Down
ALTER TABLE agent_settings DROP COLUMN cpu_limit_percent;
ALTER TABLE agent_settings DROP COLUMN memory_limit_mb;
