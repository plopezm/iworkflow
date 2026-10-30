-- name: InsertTaskRelatedProject :exec
INSERT INTO task_related_projects (task_id, workspace_id, position)
VALUES (sqlc.arg(task_id), sqlc.arg(workspace_id), sqlc.arg(position));

-- name: ListTaskRelatedProjects :many
SELECT workspace_id
FROM task_related_projects
WHERE task_id = sqlc.arg(task_id)
ORDER BY position ASC;

-- name: DeleteTaskRelatedProjects :exec
DELETE FROM task_related_projects WHERE task_id = sqlc.arg(task_id);
