UPDATE jira_lanes
SET jql = 'assignee = currentUser() AND status in ("To Do", "In Progress", "In Review") ORDER BY updated DESC'
WHERE id = 'mine';
