-- +goose Up
CREATE TABLE workspace_groups (
    id TEXT NOT NULL PRIMARY KEY,
    name TEXT NOT NULL,
    color TEXT NULL,
    collapsed INTEGER NOT NULL DEFAULT 0,
    position INTEGER NOT NULL
);

-- A workspace belongs to at most one group. Deleting a group only drops the memberships.
CREATE TABLE workspace_group_members (
    workspace_id TEXT NOT NULL PRIMARY KEY REFERENCES workspaces(id) ON DELETE CASCADE,
    group_id TEXT NOT NULL REFERENCES workspace_groups(id) ON DELETE CASCADE
);

CREATE INDEX idx_workspace_group_members_group ON workspace_group_members(group_id);

CREATE TABLE workspace_pins (
    workspace_id TEXT NOT NULL PRIMARY KEY REFERENCES workspaces(id) ON DELETE CASCADE,
    position INTEGER NOT NULL
);

-- +goose Down
DROP TABLE workspace_pins;
DROP INDEX idx_workspace_group_members_group;
DROP TABLE workspace_group_members;
DROP TABLE workspace_groups;
