-- name: InsertFailedAlert :exec
INSERT INTO failed_alerts (event, error, failed_channel_ids) VALUES ($1, $2, $3);
