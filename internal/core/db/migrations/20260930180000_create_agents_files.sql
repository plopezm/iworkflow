-- +goose Up
CREATE TABLE agents_files (
    id TEXT PRIMARY KEY NOT NULL,
    description TEXT NOT NULL,
    path TEXT NOT NULL UNIQUE,
    created_at TIMESTAMP NOT NULL
);

-- +goose Down
DROP TABLE agents_files;
