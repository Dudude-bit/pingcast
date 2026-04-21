-- name: CreateMonitorGroup :one
INSERT INTO monitor_groups (user_id, name, ordering)
VALUES ($1, $2, $3)
RETURNING id, user_id, name, ordering, created_at;

-- name: ListMonitorGroupsByUserID :many
SELECT id, user_id, name, ordering, created_at
FROM monitor_groups
WHERE user_id = $1
ORDER BY ordering, id;

-- name: UpdateMonitorGroup :exec
UPDATE monitor_groups
SET name = $3, ordering = $4
WHERE id = $1 AND user_id = $2;

-- name: DeleteMonitorGroup :exec
DELETE FROM monitor_groups WHERE id = $1 AND user_id = $2;

-- name: AssignMonitorToGroup :exec
-- Sets monitor.group_id. Pass NULL to unassign. WHERE user_id check
-- prevents cross-tenant assignment.
UPDATE monitors
SET group_id = $3
WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL;
