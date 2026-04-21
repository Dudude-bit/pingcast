-- name: CreateMaintenanceWindow :one
INSERT INTO maintenance_windows (monitor_id, starts_at, ends_at, reason)
VALUES ($1, $2, $3, $4)
RETURNING id, monitor_id, starts_at, ends_at, reason, created_at;

-- name: ListMaintenanceWindowsByMonitorID :many
SELECT id, monitor_id, starts_at, ends_at, reason, created_at
FROM maintenance_windows
WHERE monitor_id = $1
ORDER BY starts_at DESC;

-- name: ListMaintenanceWindowsByUserID :many
SELECT mw.id, mw.monitor_id, mw.starts_at, mw.ends_at, mw.reason, mw.created_at
FROM maintenance_windows mw
JOIN monitors m ON mw.monitor_id = m.id
WHERE m.user_id = $1 AND m.deleted_at IS NULL
ORDER BY mw.starts_at DESC;

-- name: DeleteMaintenanceWindow :exec
DELETE FROM maintenance_windows mw
WHERE mw.id = $1
  AND mw.monitor_id IN (SELECT m.id FROM monitors m WHERE m.user_id = $2);

-- name: HasActiveMaintenanceWindow :one
SELECT EXISTS(
    SELECT 1 FROM maintenance_windows
    WHERE monitor_id = $1
      AND starts_at <= NOW()
      AND ends_at   >  NOW()
)::bool AS is_active;
