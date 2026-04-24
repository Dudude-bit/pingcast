-- +goose Up
-- Add soft delete columns
ALTER TABLE users ADD COLUMN deleted_at TIMESTAMPTZ;
ALTER TABLE monitors ADD COLUMN deleted_at TIMESTAMPTZ;
ALTER TABLE notification_channels ADD COLUMN deleted_at TIMESTAMPTZ;

-- Partial indexes for active records (WHERE deleted_at IS NULL)
CREATE INDEX idx_users_active ON users (id) WHERE deleted_at IS NULL;
CREATE INDEX idx_monitors_active ON monitors (user_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_channels_active ON notification_channels (user_id) WHERE deleted_at IS NULL;
