-- name: UpsertHookRun :exec
INSERT INTO task_hook_runs (id, task_id, hook_id, phase, event, status, exit_code, output, started_at, finished_at)
VALUES (
    sqlc.arg(id), sqlc.arg(task_id), sqlc.arg(hook_id), sqlc.arg(phase), sqlc.arg(event), sqlc.arg(status),
    sqlc.arg(exit_code), sqlc.arg(output), sqlc.arg(started_at), sqlc.arg(finished_at)
)
ON CONFLICT (id) DO UPDATE SET
    status = excluded.status, exit_code = excluded.exit_code, output = excluded.output,
    finished_at = excluded.finished_at;

-- name: ListHookRunsByTask :many
SELECT id, task_id, hook_id, phase, event, status, exit_code, output, started_at, finished_at
FROM task_hook_runs
WHERE task_id = sqlc.arg(task_id)
ORDER BY started_at ASC, rowid ASC;
