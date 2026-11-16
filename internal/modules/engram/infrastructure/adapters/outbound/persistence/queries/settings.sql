-- name: GetSetting :one
SELECT value
FROM app_preferences
WHERE key = sqlc.arg(key);

-- name: UpsertSetting :exec
INSERT INTO app_preferences (key, value)
VALUES (sqlc.arg(key), sqlc.arg(value))
ON CONFLICT (key) DO UPDATE SET value = excluded.value;
