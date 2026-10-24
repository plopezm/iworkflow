-- name: GetCodeHostSettings :one
SELECT provider, base_url, token, insecure_tls
FROM code_host_settings
WHERE provider = sqlc.arg(provider);

-- name: ListCodeHostSettings :many
SELECT provider, base_url, token, insecure_tls
FROM code_host_settings
ORDER BY provider ASC;

-- name: ReplaceCodeHostSettings :exec
REPLACE INTO code_host_settings (provider, base_url, token, insecure_tls)
VALUES (sqlc.arg(provider), sqlc.arg(base_url), sqlc.arg(token), sqlc.arg(insecure_tls));
