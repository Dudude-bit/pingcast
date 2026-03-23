-- New indexes (replace existing without DESC)
DROP INDEX IF EXISTS idx_check_results_monitor_checked;
CREATE INDEX idx_check_results_monitor_checked ON check_results (monitor_id, checked_at DESC);
CREATE INDEX IF NOT EXISTS idx_monitor_channels_channel ON monitor_channels (channel_id);
-- monitor_channels(monitor_id) NOT needed: covered by PK (monitor_id, channel_id)

-- Replace existing incidents index (add DESC for ORDER BY optimization)
DROP INDEX IF EXISTS idx_incidents_monitor_started;
CREATE INDEX idx_incidents_monitor_started ON incidents (monitor_id, started_at DESC);
-- sessions(expires_at) already exists in migration 005
