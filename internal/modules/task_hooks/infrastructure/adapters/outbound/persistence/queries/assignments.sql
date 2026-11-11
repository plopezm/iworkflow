-- name: ListWorkspaceAssignments :many
SELECT
    a.id AS assignment_id, a.hook_id, a.workspace_id, a.event, a.phase, a.position, a.enabled, a.on_failure,
    h.name, h.kind, h.timeout_seconds, h.command, h.args, h.env, h.use_shell, h.working_dir,
    h.prompt, h.runtime, h.model, h.thinking, h.profile
FROM hook_assignments a
JOIN hooks h ON h.id = a.hook_id
WHERE a.workspace_id = sqlc.arg(workspace_id)
ORDER BY a.event ASC, a.phase ASC, a.position ASC;

-- name: ListAssignmentsFor :many
SELECT
    a.id AS assignment_id, a.hook_id, a.workspace_id, a.event, a.phase, a.position, a.enabled, a.on_failure,
    h.name, h.kind, h.timeout_seconds, h.command, h.args, h.env, h.use_shell, h.working_dir,
    h.prompt, h.runtime, h.model, h.thinking, h.profile
FROM hook_assignments a
JOIN hooks h ON h.id = a.hook_id
WHERE a.workspace_id = sqlc.arg(workspace_id) AND a.event = sqlc.arg(event) AND a.phase = sqlc.arg(phase)
ORDER BY a.position ASC;

-- name: ListAssignmentsByHook :many
SELECT * FROM hook_assignments WHERE hook_id = sqlc.arg(hook_id) ORDER BY workspace_id ASC, event ASC, phase ASC;

-- name: GetAssignment :one
SELECT * FROM hook_assignments WHERE id = sqlc.arg(id);

-- name: UpsertAssignment :exec
INSERT INTO hook_assignments (id, hook_id, workspace_id, event, phase, position, enabled, on_failure)
VALUES (
    sqlc.arg(id), sqlc.arg(hook_id), sqlc.arg(workspace_id), sqlc.arg(event), sqlc.arg(phase),
    (SELECT COALESCE(MAX(position), 0) + 1 FROM hook_assignments
        WHERE workspace_id = sqlc.arg(workspace_id) AND event = sqlc.arg(event) AND phase = sqlc.arg(phase)),
    sqlc.arg(enabled), sqlc.arg(on_failure)
)
ON CONFLICT (id) DO UPDATE SET
    hook_id = excluded.hook_id, workspace_id = excluded.workspace_id, event = excluded.event,
    phase = excluded.phase, enabled = excluded.enabled, on_failure = excluded.on_failure;

-- name: DeleteAssignment :execrows
DELETE FROM hook_assignments WHERE id = sqlc.arg(id);

-- name: SetAssignmentPosition :exec
UPDATE hook_assignments SET position = sqlc.arg(position)
WHERE id = sqlc.arg(id) AND workspace_id = sqlc.arg(workspace_id) AND event = sqlc.arg(event) AND phase = sqlc.arg(phase);
