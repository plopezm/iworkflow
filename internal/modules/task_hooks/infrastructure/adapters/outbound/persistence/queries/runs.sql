-- name: UpsertHookRun :exec
INSERT INTO task_hook_runs (
    id, task_id, hook_id, assignment_id, hook_name, workspace_id, phase, event, status, exit_code, output,
    started_at, finished_at
)
VALUES (
    sqlc.arg(id), sqlc.arg(task_id), sqlc.arg(hook_id), sqlc.arg(assignment_id), sqlc.arg(hook_name),
    sqlc.arg(workspace_id), sqlc.arg(phase), sqlc.arg(event), sqlc.arg(status),
    sqlc.arg(exit_code), sqlc.arg(output), sqlc.arg(started_at), sqlc.arg(finished_at)
)
ON CONFLICT (id) DO UPDATE SET
    status = excluded.status, exit_code = excluded.exit_code, output = excluded.output,
    finished_at = excluded.finished_at;

-- name: ListHookRunsByTask :many
SELECT * FROM task_hook_runs
WHERE task_id = sqlc.arg(task_id)
ORDER BY started_at ASC, rowid ASC;

-- name: ListHookRuns :many
SELECT * FROM task_hook_runs
WHERE (sqlc.arg(hook_id) = '' OR hook_id = sqlc.arg(hook_id))
  AND (sqlc.arg(workspace_id) = '' OR workspace_id = sqlc.arg(workspace_id))
  AND (sqlc.arg(failed_only) = 0 OR status NOT IN ('ok', 'skipped', 'running'))
ORDER BY started_at DESC, rowid DESC
LIMIT sqlc.arg(row_limit);

-- name: ListLastFailedRuns :many
SELECT r.* FROM task_hook_runs r
WHERE r.status NOT IN ('ok', 'skipped', 'running')
  AND r.rowid = (
      SELECT x.rowid FROM task_hook_runs x
      WHERE x.hook_id = r.hook_id AND x.status NOT IN ('ok', 'skipped', 'running')
      ORDER BY x.started_at DESC, x.rowid DESC LIMIT 1
  );

-- name: FailStaleRunning :execrows
UPDATE task_hook_runs
SET status = 'error', output = sqlc.arg(output), finished_at = sqlc.arg(finished_at)
WHERE status = 'running';
