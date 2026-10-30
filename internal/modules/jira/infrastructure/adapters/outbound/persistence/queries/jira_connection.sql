-- name: GetJiraConnection :one
SELECT id, base_url, token, accept_invalid_certs, last_sync_at, last_sync_error, updated_at
FROM jira_connection
WHERE id = sqlc.arg(id);

-- name: UpsertJiraConnection :exec
INSERT INTO jira_connection (id, base_url, token, accept_invalid_certs, last_sync_at, last_sync_error, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(base_url), sqlc.arg(token), sqlc.arg(accept_invalid_certs),
        sqlc.narg(last_sync_at), sqlc.narg(last_sync_error), sqlc.arg(updated_at))
ON CONFLICT (id) DO UPDATE SET
    base_url = excluded.base_url,
    token = excluded.token,
    accept_invalid_certs = excluded.accept_invalid_certs,
    -- Sync bookkeeping belongs to a server: it is kept only while the URL is unchanged.
    last_sync_at = CASE WHEN jira_connection.base_url = excluded.base_url
                        THEN jira_connection.last_sync_at ELSE excluded.last_sync_at END,
    last_sync_error = CASE WHEN jira_connection.base_url = excluded.base_url
                           THEN jira_connection.last_sync_error ELSE excluded.last_sync_error END,
    updated_at = excluded.updated_at;

-- name: RecordJiraSync :execrows
UPDATE jira_connection
SET last_sync_at = sqlc.arg(last_sync_at), last_sync_error = sqlc.narg(last_sync_error)
WHERE id = sqlc.arg(id);

-- name: DeleteJiraConnection :exec
DELETE FROM jira_connection;
