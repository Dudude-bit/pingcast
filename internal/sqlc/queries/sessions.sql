-- name: CreateSession :one
INSERT INTO sessions (id, user_id, expires_at)
VALUES ($1, $2, $3)
RETURNING id, user_id, expires_at, created_at;

-- name: GetSessionByID :one
SELECT id, user_id, expires_at, created_at
FROM sessions
WHERE id = $1 AND expires_at > NOW();

-- name: TouchSession :exec
UPDATE sessions SET expires_at = $2 WHERE id = $1;

-- name: DeleteSession :exec
DELETE FROM sessions WHERE id = $1;

-- name: DeleteExpiredSessions :execrows
DELETE FROM sessions WHERE expires_at < NOW();
