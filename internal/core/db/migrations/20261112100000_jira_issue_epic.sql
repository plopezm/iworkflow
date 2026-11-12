-- +goose Up
ALTER TABLE jira_issues ADD COLUMN epic_key TEXT;

CREATE INDEX jira_issues_epic_key_idx ON jira_issues (epic_key);

-- +goose Down
DROP INDEX jira_issues_epic_key_idx;
ALTER TABLE jira_issues DROP COLUMN epic_key;
