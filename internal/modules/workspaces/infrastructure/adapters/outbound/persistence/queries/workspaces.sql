-- name: InsertWorkspace :exec
INSERT INTO workspaces (id, name, path, base_branch, created_at)
VALUES (sqlc.arg(id), sqlc.arg(name), sqlc.arg(path), sqlc.arg(base_branch), sqlc.arg(created_at));

-- name: ListWorkspaces :many
SELECT id, name, path, base_branch, created_at
FROM workspaces
ORDER BY created_at ASC;

-- name: UpdateWorkspaceNameAndBranch :execrows
UPDATE workspaces
SET name = sqlc.arg(name), base_branch = sqlc.arg(base_branch)
WHERE id = sqlc.arg(id);

-- name: GetWorkspaceByID :one
SELECT id, name, path, base_branch, created_at
FROM workspaces
WHERE id = sqlc.arg(id);

-- name: GetWorkspaceByPath :one
SELECT id, name, path, base_branch, created_at
FROM workspaces
WHERE path = sqlc.arg(path);

-- name: DeleteWorkspace :execrows
DELETE FROM workspaces
WHERE id = sqlc.arg(id);
