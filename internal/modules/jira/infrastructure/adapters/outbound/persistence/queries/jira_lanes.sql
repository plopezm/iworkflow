-- name: ListJiraLanes :many
SELECT id, name, jql, is_default, collapsed, position, created_at
FROM jira_lanes
ORDER BY position ASC, created_at ASC;

-- name: GetJiraLane :one
SELECT id, name, jql, is_default, collapsed, position, created_at
FROM jira_lanes
WHERE id = sqlc.arg(id);

-- name: InsertJiraLaneAtEnd :execrows
-- The position is computed in the same statement, so two creations cannot take the same slot.
INSERT INTO jira_lanes (id, name, jql, is_default, collapsed, position, created_at)
SELECT sqlc.arg(id), sqlc.arg(name), sqlc.arg(jql), 0, 0, COALESCE(MAX(position), 0) + 1, sqlc.arg(created_at)
FROM jira_lanes;

-- name: UpdateJiraLaneDefinition :execrows
UPDATE jira_lanes
SET name = sqlc.arg(name), jql = sqlc.arg(jql)
WHERE id = sqlc.arg(id) AND is_default = 0;

-- name: SetJiraLaneCollapsed :execrows
UPDATE jira_lanes
SET collapsed = sqlc.arg(collapsed)
WHERE id = sqlc.arg(id);

-- name: DeleteCustomJiraLane :execrows
DELETE FROM jira_lanes
WHERE id = sqlc.arg(id) AND is_default = 0;
