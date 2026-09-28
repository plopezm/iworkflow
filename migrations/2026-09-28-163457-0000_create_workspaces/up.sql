CREATE TABLE workspaces (
    id TEXT PRIMARY KEY NOT NULL,
    name TEXT NOT NULL,
    path TEXT NOT NULL UNIQUE,
    base_branch TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL
);
