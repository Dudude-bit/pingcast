-- name: CreateChannel :one
INSERT INTO notification_channels (user_id, name, type, config)
VALUES ($1, $2, $3, $4)
RETURNING id, user_id, name, type, config, is_enabled, created_at;

-- name: GetChannelByID :one
SELECT id, user_id, name, type, config, is_enabled, created_at
FROM notification_channels WHERE id = $1;

-- name: ListChannelsByUserID :many
SELECT id, user_id, name, type, config, is_enabled, created_at
FROM notification_channels WHERE user_id = $1 ORDER BY created_at;

-- name: ListChannelsForMonitor :many
SELECT c.id, c.user_id, c.name, c.type, c.config, c.is_enabled, c.created_at
FROM notification_channels c
JOIN monitor_channels mc ON c.id = mc.channel_id
WHERE mc.monitor_id = $1
ORDER BY c.name;

-- name: UpdateChannel :exec
UPDATE notification_channels
SET name = $2, config = $3, is_enabled = $4
WHERE id = $1 AND user_id = $5;

-- name: DeleteChannel :exec
DELETE FROM notification_channels WHERE id = $1 AND user_id = $2;

-- name: BindChannelToMonitor :exec
INSERT INTO monitor_channels (monitor_id, channel_id) VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: UnbindChannelFromMonitor :exec
DELETE FROM monitor_channels WHERE monitor_id = $1 AND channel_id = $2;
