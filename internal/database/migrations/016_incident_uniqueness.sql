-- +goose Up
-- Prevent duplicate active incidents for the same monitor (Issue 4.6).
-- If duplicates exist, resolve them first (keep the earliest).
UPDATE incidents i
SET resolved_at = NOW()
WHERE resolved_at IS NULL
  AND id NOT IN (
    SELECT MIN(id) FROM incidents WHERE resolved_at IS NULL GROUP BY monitor_id
  );

CREATE UNIQUE INDEX idx_incidents_active_monitor
ON incidents (monitor_id) WHERE resolved_at IS NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_incidents_active_monitor;
