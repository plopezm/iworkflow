-- +goose Up
CREATE TABLE jira_connection (
    id INTEGER PRIMARY KEY NOT NULL CHECK (id = 1),
    base_url TEXT NOT NULL,
    token TEXT NOT NULL,
    jql TEXT NOT NULL,
    accept_invalid_certs INTEGER NOT NULL DEFAULT 0,
    last_sync_at TIMESTAMP,
    last_sync_error TEXT,
    updated_at TIMESTAMP NOT NULL
);

CREATE TABLE jira_issues (
    key TEXT PRIMARY KEY NOT NULL,
    summary TEXT NOT NULL,
    description TEXT NOT NULL,
    status TEXT NOT NULL,
    issue_type TEXT NOT NULL,
    priority TEXT,
    assignee TEXT,
    reporter TEXT,
    labels TEXT NOT NULL,
    url TEXT NOT NULL,
    jira_updated_at TIMESTAMP NOT NULL,
    local_state TEXT NOT NULL DEFAULT 'new',
    task_id TEXT,
    synced_at TIMESTAMP NOT NULL
);

CREATE INDEX jira_issues_jira_updated_at_idx ON jira_issues (jira_updated_at DESC);

-- +goose Down
DROP TABLE jira_issues;
DROP TABLE jira_connection;
