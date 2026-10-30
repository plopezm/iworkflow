-- +goose Up
ALTER TABLE agent_settings ADD COLUMN limits_disabled INTEGER NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE agent_settings DROP COLUMN limits_disabled;
