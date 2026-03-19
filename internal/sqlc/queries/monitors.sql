-- name: CreateMonitor :one
INSERT INTO monitors (user_id, name, url, method, interval_seconds, expected_status, keyword, alert_after_failures, is_paused, is_public)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING id, user_id, name, url, method, interval_seconds, expected_status, keyword, alert_after_failures, is_paused, is_public, current_status, created_at;

-- name: GetMonitorByID :one
SELECT id, user_id, name, url, method, interval_seconds, expected_status, keyword, alert_after_failures, is_paused, is_public, current_status, created_at
FROM monitors WHERE id = $1;

-- name: ListMonitorsByUserID :many
SELECT id, user_id, name, url, method, interval_seconds, expected_status, keyword, alert_after_failures, is_paused, is_public, current_status, created_at
FROM monitors WHERE user_id = $1 ORDER BY created_at;

-- name: ListPublicMonitorsByUserSlug :many
SELECT m.id, m.user_id, m.name, m.url, m.method, m.interval_seconds, m.expected_status, m.keyword, m.alert_after_failures, m.is_paused, m.is_public, m.current_status, m.created_at
FROM monitors m
JOIN users u ON m.user_id = u.id
WHERE u.slug = $1 AND m.is_public = TRUE
ORDER BY m.name;

-- name: ListActiveMonitors :many
SELECT id, user_id, name, url, method, interval_seconds, expected_status, keyword, alert_after_failures, is_paused, is_public, current_status, created_at
FROM monitors WHERE is_paused = FALSE;

-- name: CountMonitorsByUserID :one
SELECT COUNT(*)::int FROM monitors WHERE user_id = $1;

-- name: UpdateMonitor :exec
UPDATE monitors
SET name = $2, url = $3, method = $4, interval_seconds = $5, expected_status = $6,
    keyword = $7, alert_after_failures = $8, is_paused = $9, is_public = $10
WHERE id = $1 AND user_id = $11;

-- name: UpdateMonitorStatus :exec
UPDATE monitors SET current_status = $2 WHERE id = $1;

-- name: DeleteMonitor :exec
DELETE FROM monitors WHERE id = $1 AND user_id = $2;
