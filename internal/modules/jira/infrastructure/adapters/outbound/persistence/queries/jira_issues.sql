-- name: ListJiraIssues :many
SELECT key, summary, description, status, issue_type, priority, assignee, reporter, labels, url,
       jira_updated_at, local_state, task_id, synced_at
FROM jira_issues
ORDER BY jira_updated_at DESC;

-- name: GetJiraIssue :one
SELECT key, summary, description, status, issue_type, priority, assignee, reporter, labels, url,
       jira_updated_at, local_state, task_id, synced_at
FROM jira_issues
WHERE key = sqlc.arg(key);

-- name: InsertJiraIssueIfAbsent :execrows
INSERT INTO jira_issues (key, summary, description, status, issue_type, priority, assignee, reporter, labels, url,
                         jira_updated_at, local_state, task_id, synced_at)
VALUES (sqlc.arg(key), sqlc.arg(summary), sqlc.arg(description), sqlc.arg(status), sqlc.arg(issue_type),
        sqlc.narg(priority), sqlc.narg(assignee), sqlc.narg(reporter), sqlc.arg(labels), sqlc.arg(url),
        sqlc.arg(jira_updated_at), sqlc.arg(local_state), NULL, sqlc.arg(synced_at))
ON CONFLICT (key) DO NOTHING;

-- name: RefreshJiraIssueRemote :execrows
UPDATE jira_issues
SET summary = sqlc.arg(summary),
    description = sqlc.arg(description),
    status = sqlc.arg(status),
    issue_type = sqlc.arg(issue_type),
    priority = sqlc.narg(priority),
    assignee = sqlc.narg(assignee),
    reporter = sqlc.narg(reporter),
    labels = sqlc.arg(labels),
    url = sqlc.arg(url),
    jira_updated_at = sqlc.arg(jira_updated_at),
    synced_at = sqlc.arg(synced_at)
WHERE key = sqlc.arg(key);

-- name: SetJiraIssueLocalState :execrows
UPDATE jira_issues
SET local_state = sqlc.arg(local_state), task_id = sqlc.narg(task_id)
WHERE key = sqlc.arg(key);

-- name: DeleteAllJiraIssues :exec
DELETE FROM jira_issues;

-- name: DeleteRetiredNewJiraIssues :exec
DELETE FROM jira_issues
WHERE local_state = sqlc.arg(new_state)
  AND key NOT IN (SELECT issue_key FROM jira_lane_issues);

-- name: ListJiraLaneMemberships :many
SELECT lane_id, issue_key
FROM jira_lane_issues;

-- name: DeleteJiraLaneMemberships :exec
DELETE FROM jira_lane_issues
WHERE lane_id = sqlc.arg(lane_id);

-- name: InsertJiraLaneMembership :exec
INSERT INTO jira_lane_issues (lane_id, issue_key)
VALUES (sqlc.arg(lane_id), sqlc.arg(issue_key));
