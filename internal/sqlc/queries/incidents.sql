-- name: CreateIncident :one
INSERT INTO incidents (monitor_id, cause)
VALUES ($1, $2)
RETURNING *;

-- name: ResolveIncident :exec
UPDATE incidents SET resolved_at = $2 WHERE id = $1;

-- name: GetOpenIncidentByMonitorID :one
SELECT * FROM incidents
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
SELECT * FROM incidents
WHERE monitor_id = $1
ORDER BY started_at DESC
LIMIT $2;
