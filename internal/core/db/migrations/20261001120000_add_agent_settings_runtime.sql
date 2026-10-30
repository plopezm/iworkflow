-- +goose Up
ALTER TABLE agent_settings ADD COLUMN runtime TEXT NULL;

-- +goose Down
ALTER TABLE agent_settings DROP COLUMN runtime;
