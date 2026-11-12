-- +goose Up
CREATE TABLE jira_worklogs (
    id INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
    task_id TEXT NOT NULL,
    issue_key TEXT NOT NULL,
    jira_worklog_id TEXT NOT NULL,
    seconds INTEGER NOT NULL,
    comment TEXT NOT NULL,
    started_at TIMESTAMP NOT NULL,
    logged_at TIMESTAMP NOT NULL
);

CREATE INDEX idx_jira_worklogs_task ON jira_worklogs (task_id, logged_at);
CREATE INDEX idx_jira_issues_task_id ON jira_issues (task_id);

-- +goose Down
DROP INDEX idx_jira_issues_task_id;
DROP INDEX idx_jira_worklogs_task;
DROP TABLE jira_worklogs;
