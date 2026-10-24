-- name: InsertTask :exec
INSERT INTO tasks (
    id, workspace_id, title, prompt, branch, base_branch, worktree_path, status, error,
    created_at, updated_at, pending_dialogs, subagents_active, subagents_waiting, priority,
    base_sha, head_sha, memory_limit_mb, cpu_limit_percent, resume_at, resume_attempts
) VALUES (
    sqlc.arg(id), sqlc.arg(workspace_id), sqlc.arg(title), sqlc.arg(prompt), sqlc.arg(branch),
    sqlc.arg(base_branch), sqlc.arg(worktree_path), sqlc.arg(status), sqlc.narg(error),
    sqlc.arg(created_at), sqlc.arg(updated_at), sqlc.arg(pending_dialogs),
    sqlc.arg(subagents_active), sqlc.arg(subagents_waiting), sqlc.arg(priority),
    sqlc.narg(base_sha), sqlc.narg(head_sha), sqlc.narg(memory_limit_mb), sqlc.narg(cpu_limit_percent),
    sqlc.narg(resume_at), sqlc.arg(resume_attempts)
);

-- name: GetTaskByID :one
SELECT id, workspace_id, title, prompt, branch, base_branch, worktree_path, status, error,
       created_at, updated_at, pending_dialogs, subagents_active, subagents_waiting, priority,
       base_sha, head_sha, memory_limit_mb, cpu_limit_percent, resume_at, resume_attempts
FROM tasks
WHERE id = sqlc.arg(id);

-- name: ListTasks :many
SELECT id, workspace_id, title, prompt, branch, base_branch, worktree_path, status, error,
       created_at, updated_at, pending_dialogs, subagents_active, subagents_waiting, priority,
       base_sha, head_sha, memory_limit_mb, cpu_limit_percent, resume_at, resume_attempts
FROM tasks
WHERE (sqlc.narg(workspace_id) IS NULL OR workspace_id = sqlc.narg(workspace_id))
ORDER BY created_at DESC;

-- name: ListActiveTasks :many
SELECT id, workspace_id, title, prompt, branch, base_branch, worktree_path, status, error,
       created_at, updated_at, pending_dialogs, subagents_active, subagents_waiting, priority,
       base_sha, head_sha, memory_limit_mb, cpu_limit_percent, resume_at, resume_attempts
FROM tasks
WHERE status NOT IN (SELECT value FROM json_each(sqlc.arg(closed_statuses)))
  AND (sqlc.narg(workspace_id) IS NULL OR workspace_id = sqlc.narg(workspace_id))
ORDER BY created_at DESC, id DESC;

-- name: ListDoneTasksPage :many
SELECT id, workspace_id, title, prompt, branch, base_branch, worktree_path, status, error,
       created_at, updated_at, pending_dialogs, subagents_active, subagents_waiting, priority,
       base_sha, head_sha, memory_limit_mb, cpu_limit_percent, resume_at, resume_attempts
FROM tasks
WHERE status IN (SELECT value FROM json_each(sqlc.arg(closed_statuses)))
  AND (sqlc.narg(workspace_id) IS NULL OR workspace_id = sqlc.narg(workspace_id))
ORDER BY updated_at DESC, id DESC
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: ListTasksByStatus :many
SELECT id, workspace_id, title, prompt, branch, base_branch, worktree_path, status, error,
       created_at, updated_at, pending_dialogs, subagents_active, subagents_waiting, priority,
       base_sha, head_sha, memory_limit_mb, cpu_limit_percent, resume_at, resume_attempts
FROM tasks
WHERE status IN (SELECT value FROM json_each(sqlc.arg(statuses)));

-- name: UpdateTaskStatus :execrows
UPDATE tasks
SET status = sqlc.arg(status), error = sqlc.narg(error), updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id);

-- name: ClearTaskResumeAt :exec
UPDATE tasks SET resume_at = NULL WHERE id = sqlc.arg(id);

-- name: ClearTaskSubagents :exec
UPDATE tasks SET subagents_active = 0, subagents_waiting = 0 WHERE id = sqlc.arg(id);

-- name: SetTaskSubagentActivity :execrows
UPDATE tasks
SET subagents_active = sqlc.arg(subagents_active), subagents_waiting = sqlc.arg(subagents_waiting)
WHERE id = sqlc.arg(id)
  AND status IN (SELECT value FROM json_each(sqlc.arg(live_statuses)));

-- name: SetTaskUsageLimit :exec
UPDATE tasks
SET resume_at = sqlc.narg(resume_at), resume_attempts = sqlc.arg(resume_attempts)
WHERE id = sqlc.arg(id);

-- name: SetTaskDiffRange :exec
UPDATE tasks SET base_sha = sqlc.arg(base_sha), head_sha = sqlc.arg(head_sha) WHERE id = sqlc.arg(id);

-- name: UpdateTaskDetailsIfBacklog :execrows
UPDATE tasks
SET title = sqlc.arg(title),
    prompt = sqlc.arg(prompt),
    priority = sqlc.arg(priority),
    base_branch = sqlc.arg(base_branch),
    memory_limit_mb = sqlc.narg(memory_limit_mb),
    cpu_limit_percent = sqlc.narg(cpu_limit_percent),
    updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id)
  AND status = 'backlog';

-- name: ClaimTaskForStart :execrows
UPDATE tasks
SET branch = sqlc.arg(branch),
    worktree_path = sqlc.arg(worktree_path),
    base_branch = sqlc.arg(base_branch),
    status = 'starting',
    updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id)
  AND status = 'backlog';

-- name: RevertTaskStartClaim :execrows
UPDATE tasks
SET status = 'backlog', updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id)
  AND status = 'starting';

-- name: ResetTaskToBacklog :execrows
UPDATE tasks
SET branch = '',
    worktree_path = '',
    status = 'backlog',
    error = NULL,
    pending_dialogs = '[]',
    subagents_active = 0,
    subagents_waiting = 0,
    updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id)
  AND status IN (SELECT value FROM json_each(sqlc.arg(resettable_statuses)));

-- name: UpdateTaskPendingDialogs :exec
UPDATE tasks SET pending_dialogs = sqlc.arg(pending_dialogs), updated_at = sqlc.arg(updated_at) WHERE id = sqlc.arg(id);

-- name: DeleteTaskIfBacklog :execrows
DELETE FROM tasks WHERE id = sqlc.arg(id) AND status = 'backlog';

-- name: ListTaskIDStatusesByWorkspace :many
SELECT id, status FROM tasks WHERE workspace_id = sqlc.arg(workspace_id);

-- name: DeleteTasksByWorkspace :exec
DELETE FROM tasks WHERE workspace_id = sqlc.arg(workspace_id);
