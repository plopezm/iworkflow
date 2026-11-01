-- name: ListLaunchers :many
SELECT id, name, path, args, env, position
FROM open_with_launchers
ORDER BY position ASC;

-- name: GetLauncher :one
SELECT id, name, path, args, env, position
FROM open_with_launchers
WHERE id = sqlc.arg(id);

-- name: InsertLauncher :exec
INSERT INTO open_with_launchers (id, name, path, args, env, position)
VALUES (
    sqlc.arg(id), sqlc.arg(name), sqlc.arg(path), sqlc.arg(args), sqlc.arg(env),
    (SELECT COALESCE(MAX(position), 0) + 1 FROM open_with_launchers)
);

-- name: DeleteLauncher :execrows
DELETE FROM open_with_launchers WHERE id = sqlc.arg(id);
