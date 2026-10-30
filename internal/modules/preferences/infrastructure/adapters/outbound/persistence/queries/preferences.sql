-- name: GetPreference :one
SELECT key, value
FROM app_preferences
WHERE key = sqlc.arg(key);

-- name: UpsertPreference :exec
INSERT INTO app_preferences (key, value)
VALUES (sqlc.arg(key), sqlc.arg(value))
ON CONFLICT (key) DO UPDATE SET value = excluded.value;
