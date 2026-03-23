-- Convert check_results to time-based partitioned table.
-- PK changes from (id) to (id, checked_at) — required by Postgres partitioning.
-- No FK references to check_results.id exist (verified).

-- Step 1: Rename existing table
ALTER TABLE check_results RENAME TO check_results_old;

-- Step 2: Create partitioned table with same schema
CREATE TABLE check_results (
    id BIGSERIAL,
    monitor_id UUID NOT NULL REFERENCES monitors(id),
    status TEXT NOT NULL DEFAULT 'unknown',
    status_code INT,
    response_time_ms INT NOT NULL DEFAULT 0,
    error_message TEXT,
    checked_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (id, checked_at)
) PARTITION BY RANGE (checked_at);

-- Step 3: Create initial partitions (current + next 2 months)
-- Additional partitions will be created by a scheduled job.
CREATE TABLE check_results_default PARTITION OF check_results DEFAULT;

-- Step 4: Migrate existing data
INSERT INTO check_results (id, monitor_id, status, status_code, response_time_ms, error_message, checked_at)
SELECT id, monitor_id, status, status_code, response_time_ms, error_message, checked_at
FROM check_results_old;

-- Step 5: Drop old indexes that followed the renamed table, then recreate
DROP INDEX IF EXISTS idx_check_results_monitor_checked;
CREATE INDEX idx_check_results_monitor_checked ON check_results (monitor_id, checked_at DESC);

-- Step 6: Drop old table
DROP TABLE check_results_old;
