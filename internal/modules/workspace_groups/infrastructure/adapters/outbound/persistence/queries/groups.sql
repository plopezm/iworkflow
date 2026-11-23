-- name: ListGroups :many
SELECT id, name, color, collapsed, position
FROM workspace_groups
ORDER BY position ASC;

-- name: InsertGroup :exec
INSERT INTO workspace_groups (id, name, color, collapsed, position)
VALUES (
    sqlc.arg(id), sqlc.arg(name), sqlc.narg(color), sqlc.arg(collapsed),
    (SELECT COALESCE(MAX(position), 0) + 1 FROM workspace_groups)
);

-- name: UpdateGroup :execrows
UPDATE workspace_groups
SET name = sqlc.arg(name), color = sqlc.narg(color), collapsed = sqlc.arg(collapsed)
WHERE id = sqlc.arg(id);

-- name: DeleteGroup :execrows
DELETE FROM workspace_groups WHERE id = sqlc.arg(id);

-- name: SetGroupCollapsed :execrows
UPDATE workspace_groups SET collapsed = sqlc.arg(collapsed) WHERE id = sqlc.arg(id);

-- name: ListMemberships :many
SELECT workspace_id, group_id FROM workspace_group_members;

-- name: UpsertMembership :exec
INSERT INTO workspace_group_members (workspace_id, group_id)
VALUES (sqlc.arg(workspace_id), sqlc.arg(group_id))
ON CONFLICT (workspace_id) DO UPDATE SET group_id = excluded.group_id;

-- name: DeleteMembership :exec
DELETE FROM workspace_group_members WHERE workspace_id = sqlc.arg(workspace_id);

-- name: ListPins :many
SELECT workspace_id FROM workspace_pins ORDER BY position ASC;

-- name: InsertPin :exec
INSERT INTO workspace_pins (workspace_id, position)
VALUES (sqlc.arg(workspace_id), (SELECT COALESCE(MAX(position), 0) + 1 FROM workspace_pins))
ON CONFLICT (workspace_id) DO NOTHING;

-- name: DeletePin :exec
DELETE FROM workspace_pins WHERE workspace_id = sqlc.arg(workspace_id);
