ALTER TABLE jira_connection DROP COLUMN jql;

CREATE TABLE jira_lanes (
    id TEXT PRIMARY KEY NOT NULL,
    name TEXT NOT NULL,
    jql TEXT NOT NULL,
    is_default INTEGER NOT NULL DEFAULT 0,
    collapsed INTEGER NOT NULL DEFAULT 0,
    position INTEGER NOT NULL,
    created_at TIMESTAMP NOT NULL
);

INSERT INTO jira_lanes (id, name, jql, is_default, collapsed, position, created_at)
VALUES (
    'mine',
    'Mis tareas',
    'assignee = currentUser() AND status in ("To Do", "In Progress", "In Review") ORDER BY updated DESC',
    1,
    0,
    0,
    CURRENT_TIMESTAMP
);

CREATE TABLE jira_lane_issues (
    lane_id TEXT NOT NULL REFERENCES jira_lanes (id) ON DELETE CASCADE,
    issue_key TEXT NOT NULL REFERENCES jira_issues (key) ON DELETE CASCADE,
    PRIMARY KEY (lane_id, issue_key)
);

CREATE INDEX jira_lane_issues_issue_key_idx ON jira_lane_issues (issue_key);

-- Issues imported before lanes existed belong to the default lane until the next sync.
INSERT INTO jira_lane_issues (lane_id, issue_key) SELECT 'mine', key FROM jira_issues;
