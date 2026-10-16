DROP INDEX jira_lane_issues_issue_key_idx;
DROP TABLE jira_lane_issues;
DROP TABLE jira_lanes;

ALTER TABLE jira_connection ADD COLUMN jql TEXT NOT NULL DEFAULT 'assignee = currentUser() AND resolution = Unresolved ORDER BY updated DESC';
