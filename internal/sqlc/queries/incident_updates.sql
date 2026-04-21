-- name: CreateIncidentUpdate :one
INSERT INTO incident_updates (incident_id, state, body, posted_by_user_id)
VALUES ($1, $2, $3, $4)
RETURNING id, incident_id, state, body, posted_by_user_id, posted_at;

-- name: ListIncidentUpdates :many
SELECT id, incident_id, state, body, posted_by_user_id, posted_at
FROM incident_updates
WHERE incident_id = $1
ORDER BY posted_at DESC;
