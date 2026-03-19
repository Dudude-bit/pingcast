-- name: InsertCheckResult :one
INSERT INTO check_results (monitor_id, status, status_code, response_time_ms, error_message, checked_at)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetLatestCheckResults :many
SELECT * FROM check_results
WHERE monitor_id = $1
ORDER BY checked_at DESC
LIMIT $2;

-- name: ConsecutiveFailures :one
WITH ordered AS (
    SELECT status, ROW_NUMBER() OVER (ORDER BY checked_at DESC) AS rn
    FROM check_results WHERE monitor_id = $1
),
first_up AS (
    SELECT MIN(rn) AS rn FROM ordered WHERE status = 'up'
)
SELECT COUNT(*)::int FROM ordered
WHERE status = 'down'
AND rn < COALESCE((SELECT rn FROM first_up), (SELECT COUNT(*) + 1 FROM ordered));

-- name: GetAggregatedCheckResults :many
SELECT
    date_trunc('hour', checked_at) + (EXTRACT(minute FROM checked_at)::INT / sqlc.arg(interval_minutes)::INT) * INTERVAL '1 minute' * sqlc.arg(interval_minutes)::INT AS bucket,
    AVG(response_time_ms)::FLOAT AS avg_response_ms,
    COUNT(*)::int AS check_count
FROM check_results
WHERE monitor_id = $1 AND checked_at >= $2
GROUP BY bucket
ORDER BY bucket;

-- name: GetUptimePercent :one
SELECT
    CASE WHEN COUNT(*) = 0 THEN 100.0
    ELSE (COUNT(*) FILTER (WHERE status = 'up'))::FLOAT / COUNT(*)::FLOAT * 100
    END AS uptime_percent
FROM check_results
WHERE monitor_id = $1 AND checked_at >= $2;

-- name: DeleteCheckResultsOlderThan :execrows
DELETE FROM check_results WHERE checked_at < $1;
