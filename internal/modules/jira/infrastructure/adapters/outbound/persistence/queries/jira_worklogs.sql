-- name: InsertJiraWorklog :exec
INSERT INTO jira_worklogs (task_id, issue_key, jira_worklog_id, seconds, comment, started_at, logged_at)
VALUES (sqlc.arg(task_id), sqlc.arg(issue_key), sqlc.arg(jira_worklog_id), sqlc.arg(seconds),
        sqlc.arg(comment), sqlc.arg(started_at), sqlc.arg(logged_at));

-- name: ListJiraWorklogsByTask :many
SELECT task_id, issue_key, jira_worklog_id, seconds, comment, started_at, logged_at
FROM jira_worklogs
WHERE task_id = sqlc.arg(task_id)
ORDER BY id ASC;
