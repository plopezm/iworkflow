-- +goose Up
ALTER TABLE agent_settings ADD COLUMN thinking TEXT NULL;

-- +goose Down
ALTER TABLE agent_settings DROP COLUMN thinking;
