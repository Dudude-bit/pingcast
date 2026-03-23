-- name: UpsertUptimeHourly :exec
INSERT INTO monitor_uptime_hourly (monitor_id, hour, total_checks, successful_checks)
VALUES ($1, date_trunc('hour', $2::timestamptz), 1, $3::int)
ON CONFLICT (monitor_id, hour)
DO UPDATE SET
    total_checks = monitor_uptime_hourly.total_checks + 1,
    successful_checks = monitor_uptime_hourly.successful_checks + EXCLUDED.successful_checks;

-- name: GetUptimeFromHourly :one
SELECT
    CASE WHEN SUM(total_checks) = 0 THEN 0
         ELSE (SUM(successful_checks)::float / SUM(total_checks)::float) * 100
    END AS uptime_percent
FROM monitor_uptime_hourly
WHERE monitor_id = $1 AND hour >= $2;

-- name: GetUptimeBatchFromHourly :many
SELECT
    monitor_id,
    CASE WHEN SUM(total_checks) = 0 THEN 0
         ELSE (SUM(successful_checks)::float / SUM(total_checks)::float) * 100
    END AS uptime_percent
FROM monitor_uptime_hourly
WHERE monitor_id = ANY($1::uuid[]) AND hour >= $2
GROUP BY monitor_id;
