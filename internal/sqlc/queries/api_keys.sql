-- name: CreateAPIKey :one
INSERT INTO api_keys (user_id, key_hash, name, scopes, expires_at)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, user_id, key_hash, name, scopes, created_at, last_used_at, expires_at;

-- name: GetAPIKeyByHash :one
SELECT id, user_id, key_hash, name, scopes, created_at, last_used_at, expires_at
FROM api_keys
WHERE key_hash = $1;

-- name: ListAPIKeysByUser :many
SELECT id, user_id, key_hash, name, scopes, created_at, last_used_at, expires_at
FROM api_keys
WHERE user_id = $1
ORDER BY created_at DESC;

-- name: TouchAPIKey :exec
UPDATE api_keys SET last_used_at = NOW() WHERE id = $1;

-- name: DeleteAPIKey :exec
DELETE FROM api_keys WHERE id = $1 AND user_id = $2;
