-- name: InsertTaskEvent :one
INSERT INTO task_events (task_id, payload, created_at)
VALUES (sqlc.arg(task_id), sqlc.arg(payload), sqlc.arg(created_at))
RETURNING seq, task_id, payload, created_at;

-- name: ListTaskEventsAfter :many
SELECT seq, task_id, payload, created_at
FROM task_events
WHERE task_id = sqlc.arg(task_id)
  AND seq > sqlc.arg(after_seq)
ORDER BY seq ASC;

-- name: DeleteTaskEvents :exec
DELETE FROM task_events WHERE task_id = sqlc.arg(task_id);
