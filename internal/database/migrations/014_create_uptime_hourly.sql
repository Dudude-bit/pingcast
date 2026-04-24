-- +goose Up
CREATE TABLE monitor_uptime_hourly (
    monitor_id UUID NOT NULL REFERENCES monitors(id),
    hour TIMESTAMPTZ NOT NULL,
    total_checks INT NOT NULL DEFAULT 0,
    successful_checks INT NOT NULL DEFAULT 0,
    PRIMARY KEY (monitor_id, hour)
);

CREATE INDEX idx_uptime_hourly_monitor_hour ON monitor_uptime_hourly (monitor_id, hour DESC);
