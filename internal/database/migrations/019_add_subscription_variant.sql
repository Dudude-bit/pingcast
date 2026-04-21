-- +goose Up
-- Track which LemonSqueezy price variant a Pro subscription landed on.
-- 'founder' counts against FOUNDER_CAP; 'retail' is the open-ended
-- $19/mo tier; 'gift' is for comped accounts (see
-- docs/marketing/bootstrap-prospects.md) and does NOT count against
-- the cap.
ALTER TABLE users ADD COLUMN subscription_variant TEXT;

CREATE INDEX idx_users_founder_variant ON users (subscription_variant)
    WHERE subscription_variant = 'founder' AND plan = 'pro';

-- +goose Down
DROP INDEX idx_users_founder_variant;
ALTER TABLE users DROP COLUMN subscription_variant;
