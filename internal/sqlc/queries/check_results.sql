-- name: InsertCheckResult :one
INSERT INTO check_results (monitor_id, status, status_code, response_time_ms, error_message, checked_at)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, monitor_id, status, status_code, response_time_ms, error_message, checked_at;

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

-- name: DeleteCheckResultsOlderThan :execrows
DELETE FROM check_results WHERE checked_at < $1;

-- name: DeleteCheckResultsByPlan :execrows
-- Plan-aware retention: Free users keep up to $1 worth of results, Pro
-- users keep up to $2 worth. Both cutoffs are absolute timestamps
-- computed in Go so the partition pruner can use them.
DELETE FROM check_results cr
USING monitors m, users u
WHERE cr.monitor_id = m.id
  AND m.user_id = u.id
  AND (
    (u.plan = 'free' AND cr.checked_at < $1)
    OR (u.plan = 'pro' AND cr.checked_at < $2)
  );

-- name: GetResponseTimeChart :many
SELECT
    date_trunc('hour', checked_at)::timestamptz AS bucket,
    AVG(response_time_ms)::float AS avg_response_ms,
    COUNT(*)::int AS check_count
FROM check_results
WHERE monitor_id = $1
  AND checked_at >= $2
GROUP BY bucket
ORDER BY bucket;
