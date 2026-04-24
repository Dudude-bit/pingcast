-- +goose Up
CREATE TABLE failed_alerts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event JSONB NOT NULL,
    error TEXT NOT NULL,
    failed_channel_ids UUID[] NOT NULL DEFAULT '{}',
    attempts INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_failed_alerts_created ON failed_alerts (created_at DESC);
