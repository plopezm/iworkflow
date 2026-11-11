-- name: ListHooks :many
SELECT * FROM hooks ORDER BY name COLLATE NOCASE ASC, id ASC;

-- name: GetHook :one
SELECT * FROM hooks WHERE id = sqlc.arg(id);

-- name: UpsertHook :exec
INSERT INTO hooks (
    id, name, kind, timeout_seconds, command, args, env, use_shell, working_dir,
    prompt, runtime, model, thinking, profile
)
VALUES (
    sqlc.arg(id), sqlc.arg(name), sqlc.arg(kind), sqlc.arg(timeout_seconds), sqlc.arg(command), sqlc.arg(args),
    sqlc.arg(env), sqlc.arg(use_shell), sqlc.arg(working_dir),
    sqlc.arg(prompt), sqlc.arg(runtime), sqlc.arg(model), sqlc.arg(thinking), sqlc.arg(profile)
)
ON CONFLICT (id) DO UPDATE SET
    name = excluded.name, kind = excluded.kind, timeout_seconds = excluded.timeout_seconds,
    command = excluded.command, args = excluded.args, env = excluded.env, use_shell = excluded.use_shell,
    working_dir = excluded.working_dir,
    prompt = excluded.prompt, runtime = excluded.runtime, model = excluded.model,
    thinking = excluded.thinking, profile = excluded.profile;

-- name: DeleteHook :execrows
DELETE FROM hooks WHERE id = sqlc.arg(id);

-- name: InsertHookSkip :exec
INSERT OR IGNORE INTO task_hook_skips (task_id, assignment_id) VALUES (sqlc.arg(task_id), sqlc.arg(assignment_id));

-- name: DeleteHookSkip :exec
DELETE FROM task_hook_skips WHERE task_id = sqlc.arg(task_id) AND assignment_id = sqlc.arg(assignment_id);

-- name: ListHookSkips :many
SELECT assignment_id FROM task_hook_skips WHERE task_id = sqlc.arg(task_id) ORDER BY assignment_id ASC;
