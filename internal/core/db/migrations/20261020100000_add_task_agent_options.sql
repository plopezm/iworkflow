-- +goose Up
ALTER TABLE tasks ADD COLUMN agent_runtime TEXT NULL;
ALTER TABLE tasks ADD COLUMN agent_model_provider TEXT NULL;
ALTER TABLE tasks ADD COLUMN agent_model_id TEXT NULL;
ALTER TABLE tasks ADD COLUMN agent_thinking TEXT NULL;
ALTER TABLE tasks ADD COLUMN agent_profile TEXT NULL;

-- +goose Down
ALTER TABLE tasks DROP COLUMN agent_profile;
ALTER TABLE tasks DROP COLUMN agent_thinking;
ALTER TABLE tasks DROP COLUMN agent_model_id;
ALTER TABLE tasks DROP COLUMN agent_model_provider;
ALTER TABLE tasks DROP COLUMN agent_runtime;
