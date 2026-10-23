-- name: FindAgentSettings :one
SELECT id, model_provider, model_id, profile, thinking, runtime, memory_limit_mb, cpu_limit_percent, limits_disabled
FROM agent_settings
WHERE id = ?;

-- name: ReplaceAgentSettings :exec
REPLACE INTO agent_settings (id, model_provider, model_id, profile, thinking, runtime, memory_limit_mb, cpu_limit_percent, limits_disabled)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);
