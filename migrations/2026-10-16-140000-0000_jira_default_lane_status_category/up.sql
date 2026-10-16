-- Jira's built-in status categories (To Do / In Progress / Done) exist on every server, whatever
-- the workflow calls its statuses. Only the non-Done ones are listed.
UPDATE jira_lanes
SET jql = 'assignee = currentUser() AND statusCategory in ("To Do", "In Progress") ORDER BY updated DESC'
WHERE id = 'mine';
