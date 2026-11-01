-- +goose Up
CREATE TABLE open_with_launchers (
    id TEXT NOT NULL PRIMARY KEY,
    name TEXT NOT NULL,
    path TEXT NOT NULL,
    args TEXT NOT NULL DEFAULT '[]',
    env TEXT NOT NULL DEFAULT '[]',
    position INTEGER NOT NULL
);

-- +goose Down
DROP TABLE open_with_launchers;
