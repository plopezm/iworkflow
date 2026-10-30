-- name: ListTaskDependencies :many
SELECT predecessor_id, successor_id, workspace_id, trigger_event, created_at
FROM task_dependencies
WHERE (sqlc.narg(workspace_id) IS NULL OR workspace_id = sqlc.narg(workspace_id))
ORDER BY created_at ASC, predecessor_id ASC, successor_id ASC;

-- name: InsertTaskDependency :exec
INSERT INTO task_dependencies (predecessor_id, successor_id, workspace_id, trigger_event, created_at)
VALUES (sqlc.arg(predecessor_id), sqlc.arg(successor_id), sqlc.arg(workspace_id), sqlc.arg(trigger_event), sqlc.arg(created_at));

-- name: DeleteTaskDependency :execrows
DELETE FROM task_dependencies
WHERE predecessor_id = sqlc.arg(predecessor_id)
  AND successor_id = sqlc.arg(successor_id);

-- name: DeleteTaskDependenciesOfTask :exec
DELETE FROM task_dependencies
WHERE predecessor_id = sqlc.arg(task_id)
   OR successor_id = sqlc.arg(task_id);

-- name: UpsertTaskPosition :exec
INSERT INTO task_positions (task_id, x, y)
SELECT id, sqlc.arg(x), sqlc.arg(y)
FROM tasks
WHERE id = sqlc.arg(task_id)
ON CONFLICT (task_id) DO UPDATE SET x = excluded.x, y = excluded.y;

-- name: DeleteTaskPosition :exec
DELETE FROM task_positions
WHERE task_id = sqlc.arg(task_id);

-- name: ListTaskPositionsOfWorkspace :many
SELECT p.task_id, p.x, p.y
FROM task_positions AS p
JOIN tasks AS t ON t.id = p.task_id
WHERE t.workspace_id = sqlc.arg(workspace_id)
ORDER BY p.task_id ASC;
