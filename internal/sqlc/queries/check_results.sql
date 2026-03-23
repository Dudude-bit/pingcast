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
