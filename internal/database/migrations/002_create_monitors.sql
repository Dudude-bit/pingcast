-- +goose Up
CREATE TABLE monitors (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    url VARCHAR(2048) NOT NULL,
    method VARCHAR(10) NOT NULL DEFAULT 'GET',
    interval_seconds INT NOT NULL DEFAULT 300,
    expected_status INT NOT NULL DEFAULT 200,
    keyword VARCHAR(255),
    alert_after_failures INT NOT NULL DEFAULT 3,
    is_paused BOOLEAN NOT NULL DEFAULT FALSE,
    is_public BOOLEAN NOT NULL DEFAULT FALSE,
    current_status VARCHAR(10) NOT NULL DEFAULT 'unknown',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_monitors_user_id ON monitors (user_id);
