-- name: CreateMonitor :one
INSERT INTO monitors (user_id, name, type, check_config, interval_seconds, alert_after_failures, is_paused, is_public)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, user_id, name, type, check_config, interval_seconds, alert_after_failures, is_paused, is_public, current_status, created_at, deleted_at;

-- name: GetMonitorByID :one
SELECT id, user_id, name, type, check_config, interval_seconds, alert_after_failures, is_paused, is_public, current_status, created_at, deleted_at
FROM monitors WHERE id = $1 AND deleted_at IS NULL;

-- name: ListMonitorsByUserID :many
SELECT id, user_id, name, type, check_config, interval_seconds, alert_after_failures, is_paused, is_public, current_status, created_at, deleted_at
FROM monitors WHERE user_id = $1 AND deleted_at IS NULL ORDER BY created_at;

-- name: ListPublicMonitorsByUserSlug :many
SELECT m.id, m.user_id, m.name, m.type, m.check_config, m.interval_seconds, m.alert_after_failures, m.is_paused, m.is_public, m.current_status, m.created_at, m.deleted_at
FROM monitors m
JOIN users u ON m.user_id = u.id
WHERE u.slug = $1 AND m.is_public = TRUE AND m.deleted_at IS NULL AND u.deleted_at IS NULL
ORDER BY m.name;

-- name: ListActiveMonitors :many
SELECT id, user_id, name, type, check_config, interval_seconds, alert_after_failures, is_paused, is_public, current_status, created_at, deleted_at
FROM monitors WHERE is_paused = FALSE AND deleted_at IS NULL;

-- name: ListProHTTPMonitors :many
-- Used by the daily SSL-expiry scan: every HTTP monitor owned by a Pro
-- user, joined to user_id for alert publication. Skips paused and
-- soft-deleted monitors.
SELECT m.id, m.user_id, m.name, m.check_config
FROM monitors m
JOIN users u ON m.user_id = u.id
WHERE u.plan = 'pro'
  AND m.type = 'http'
  AND m.is_paused = FALSE
  AND m.deleted_at IS NULL
  AND u.deleted_at IS NULL;

-- name: CountMonitorsByUserID :one
SELECT COUNT(*)::int FROM monitors WHERE user_id = $1 AND deleted_at IS NULL;

-- name: UpdateMonitor :exec
UPDATE monitors
SET name = $2, check_config = $3, interval_seconds = $4, alert_after_failures = $5, is_paused = $6, is_public = $7
WHERE id = $1 AND user_id = $8 AND deleted_at IS NULL;

-- name: UpdateMonitorStatus :one
WITH prev AS (
    SELECT m.current_status FROM monitors m WHERE m.id = $1 AND m.deleted_at IS NULL
)
UPDATE monitors SET current_status = $2
WHERE monitors.id = $1 AND monitors.deleted_at IS NULL
RETURNING (SELECT prev.current_status FROM prev) AS previous_status;

-- name: ToggleMonitorPause :one
UPDATE monitors SET is_paused = NOT is_paused
WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL
RETURNING id, user_id, name, type, check_config, interval_seconds, alert_after_failures, is_paused, is_public, current_status, created_at, deleted_at;

-- name: DeleteMonitor :exec
UPDATE monitors SET deleted_at = NOW() WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL;
