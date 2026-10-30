-- +goose Up
ALTER TABLE tasks ADD COLUMN pending_dialogs TEXT NOT NULL DEFAULT '[]';

-- +goose Down
ALTER TABLE tasks DROP COLUMN pending_dialogs;
