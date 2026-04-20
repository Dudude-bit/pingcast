-- name: GetPublicStats :one
SELECT
    (SELECT COUNT(*) FROM monitors WHERE deleted_at IS NULL)::bigint AS monitors_count,
    (SELECT COUNT(*) FROM incidents WHERE resolved_at IS NOT NULL)::bigint AS incidents_resolved,
    (SELECT COUNT(DISTINCT user_id) FROM monitors
     WHERE deleted_at IS NULL AND is_public = TRUE)::bigint AS public_status_pages;
