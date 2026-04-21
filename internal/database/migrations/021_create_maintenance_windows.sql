-- +goose Up
-- Scheduled maintenance windows: alerts are suppressed while a monitor
-- is inside one, the status page shows "Scheduled maintenance" instead
-- of "down", and no incident is opened for failures that happen during
-- the window.
CREATE TABLE maintenance_windows (
    id BIGSERIAL PRIMARY KEY,
    monitor_id UUID NOT NULL REFERENCES monitors(id) ON DELETE CASCADE,
    starts_at TIMESTAMPTZ NOT NULL,
    ends_at TIMESTAMPTZ NOT NULL,
    reason TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT maintenance_window_valid CHECK (ends_at > starts_at)
);

CREATE INDEX idx_maintenance_windows_monitor_active
    ON maintenance_windows (monitor_id, starts_at, ends_at);

-- +goose Down
DROP TABLE maintenance_windows;
