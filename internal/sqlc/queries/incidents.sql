-- name: CreateIncident :one
INSERT INTO incidents (monitor_id, cause, state, is_manual, title)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, monitor_id, started_at, resolved_at, cause, state, is_manual, title;

-- name: ResolveIncident :exec
UPDATE incidents SET resolved_at = $2, state = 'resolved' WHERE id = $1;

-- name: UpdateIncidentState :exec
UPDATE incidents SET state = $2 WHERE id = $1;

-- name: GetIncidentByID :one
SELECT id, monitor_id, started_at, resolved_at, cause, state, is_manual, title
FROM incidents WHERE id = $1;

-- name: GetOpenIncidentByMonitorID :one
SELECT id, monitor_id, started_at, resolved_at, cause, state, is_manual, title
FROM incidents
WHERE monitor_id = $1 AND resolved_at IS NULL
ORDER BY started_at DESC
LIMIT 1;

-- name: IsInCooldown :one
SELECT EXISTS(
    SELECT 1 FROM incidents
    WHERE monitor_id = $1 AND resolved_at IS NOT NULL
    AND resolved_at > NOW() - INTERVAL '5 minutes'
)::bool AS in_cooldown;

-- name: ListIncidentsByMonitorID :many
SELECT id, monitor_id, started_at, resolved_at, cause, state, is_manual, title
FROM incidents
WHERE monitor_id = $1
ORDER BY started_at DESC
LIMIT $2;

-- name: ListIncidentsByUserIDForExport :many
-- Flat incident rows joined to monitor name for CSV export. Ordered
-- newest-first so the CSV opens in spreadsheets with recent events at
-- the top.
SELECT i.id, i.monitor_id, m.name AS monitor_name, i.started_at,
       i.resolved_at, i.cause, i.state, i.is_manual, i.title
FROM incidents i
JOIN monitors m ON i.monitor_id = m.id
WHERE m.user_id = $1 AND m.deleted_at IS NULL
ORDER BY i.started_at DESC;
