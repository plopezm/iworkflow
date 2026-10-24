-- name: InsertTaskAttachment :exec
INSERT INTO task_attachments (id, task_id, position, name, mime_type, path, size_bytes, created_at)
VALUES (sqlc.arg(id), sqlc.arg(task_id), sqlc.arg(position), sqlc.arg(name), sqlc.arg(mime_type),
        sqlc.arg(path), sqlc.arg(size_bytes), sqlc.arg(created_at));

-- name: MaxTaskAttachmentPosition :one
SELECT CAST(COALESCE(MAX(position), -1) AS INTEGER) AS max_position
FROM task_attachments
WHERE task_id = sqlc.arg(task_id);

-- name: ListTaskAttachments :many
SELECT id, task_id, position, name, mime_type, path, size_bytes, created_at
FROM task_attachments
WHERE task_id = sqlc.arg(task_id)
ORDER BY position ASC;

-- name: ListTaskAttachmentsByIDs :many
SELECT id, task_id, position, name, mime_type, path, size_bytes, created_at
FROM task_attachments
WHERE task_id = sqlc.arg(task_id)
  AND id IN (SELECT value FROM json_each(sqlc.arg(ids)))
ORDER BY position ASC;

-- name: DeleteTaskAttachmentsByIDs :exec
DELETE FROM task_attachments
WHERE task_id = sqlc.arg(task_id)
  AND id IN (SELECT value FROM json_each(sqlc.arg(ids)));

-- name: DeleteTaskAttachmentsByTask :exec
DELETE FROM task_attachments WHERE task_id = sqlc.arg(task_id);
