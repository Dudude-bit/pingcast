CREATE TABLE incidents (
    id BIGSERIAL PRIMARY KEY,
    monitor_id UUID NOT NULL REFERENCES monitors(id) ON DELETE CASCADE,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at TIMESTAMPTZ,
    cause TEXT NOT NULL
);

CREATE INDEX idx_incidents_monitor_id ON incidents (monitor_id);
CREATE INDEX idx_incidents_monitor_started ON incidents (monitor_id, started_at);
