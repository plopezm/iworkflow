-- name: ListWorkspaceHooks :many
SELECT * FROM task_hooks
WHERE workspace_id = sqlc.arg(workspace_id)
ORDER BY position ASC;

-- name: ListHooksFor :many
SELECT * FROM task_hooks
WHERE workspace_id = sqlc.arg(workspace_id) AND event = sqlc.arg(event) AND phase = sqlc.arg(phase)
ORDER BY position ASC;

-- name: GetHook :one
SELECT * FROM task_hooks WHERE id = sqlc.arg(id);

-- name: UpsertHook :exec
INSERT INTO task_hooks (
    id, workspace_id, phase, event, kind, position, enabled, on_failure, timeout_seconds,
    command, args, env, use_shell, prompt, runtime, model, thinking, profile
)
VALUES (
    sqlc.arg(id), sqlc.arg(workspace_id), sqlc.arg(phase), sqlc.arg(event), sqlc.arg(kind),
    (SELECT COALESCE(MAX(position), 0) + 1 FROM task_hooks WHERE workspace_id = sqlc.arg(workspace_id)),
    sqlc.arg(enabled), sqlc.arg(on_failure), sqlc.arg(timeout_seconds),
    sqlc.arg(command), sqlc.arg(args), sqlc.arg(env), sqlc.arg(use_shell),
    sqlc.arg(prompt), sqlc.arg(runtime), sqlc.arg(model), sqlc.arg(thinking), sqlc.arg(profile)
)
ON CONFLICT (id) DO UPDATE SET
    phase = excluded.phase, event = excluded.event, kind = excluded.kind, enabled = excluded.enabled,
    on_failure = excluded.on_failure, timeout_seconds = excluded.timeout_seconds,
    command = excluded.command, args = excluded.args, env = excluded.env, use_shell = excluded.use_shell,
    prompt = excluded.prompt, runtime = excluded.runtime, model = excluded.model,
    thinking = excluded.thinking, profile = excluded.profile;

-- name: DeleteHook :execrows
DELETE FROM task_hooks WHERE id = sqlc.arg(id);

-- name: SetHookPosition :exec
UPDATE task_hooks SET position = sqlc.arg(position)
WHERE id = sqlc.arg(id) AND workspace_id = sqlc.arg(workspace_id);

-- name: InsertHookSkip :exec
INSERT OR IGNORE INTO task_hook_skips (task_id, hook_id) VALUES (sqlc.arg(task_id), sqlc.arg(hook_id));

-- name: DeleteHookSkip :exec
DELETE FROM task_hook_skips WHERE task_id = sqlc.arg(task_id) AND hook_id = sqlc.arg(hook_id);

-- name: ListHookSkips :many
SELECT hook_id FROM task_hook_skips WHERE task_id = sqlc.arg(task_id) ORDER BY hook_id ASC;
