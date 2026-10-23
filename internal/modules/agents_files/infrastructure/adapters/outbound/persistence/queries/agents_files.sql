-- name: InsertAgentsFile :exec
INSERT INTO agents_files (id, description, path, created_at)
VALUES (sqlc.arg(id), sqlc.arg(description), sqlc.arg(path), sqlc.arg(created_at));

-- name: ListAgentsFiles :many
SELECT id, description, path, created_at
FROM agents_files
ORDER BY created_at ASC;

-- name: GetAgentsFileByID :one
SELECT id, description, path, created_at
FROM agents_files
WHERE id = sqlc.arg(id);

-- name: DeleteAgentsFile :execrows
DELETE FROM agents_files
WHERE id = sqlc.arg(id);
