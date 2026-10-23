-- +goose Up
CREATE TABLE app_preferences (
    key TEXT NOT NULL PRIMARY KEY,
    value TEXT NOT NULL
);

-- +goose Down
DROP TABLE app_preferences;
