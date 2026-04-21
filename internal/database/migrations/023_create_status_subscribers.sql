-- +goose Up
-- Status-page email subscribers. Keyed on slug (not user_id) so the
-- subscribe form doesn't need auth — anyone visiting /status/<slug>
-- can subscribe. Double-opt-in via confirm_token; unsubscribe via
-- unsubscribe_token embedded in every outbound email.
CREATE TABLE status_subscribers (
    id BIGSERIAL PRIMARY KEY,
    slug TEXT NOT NULL,
    email TEXT NOT NULL,
    confirm_token TEXT NOT NULL,
    unsubscribe_token TEXT NOT NULL,
    confirmed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (slug, email)
);

CREATE INDEX idx_status_subs_slug_confirmed
    ON status_subscribers (slug, confirmed_at)
    WHERE confirmed_at IS NOT NULL;
CREATE UNIQUE INDEX idx_status_subs_confirm_token
    ON status_subscribers (confirm_token);
CREATE UNIQUE INDEX idx_status_subs_unsubscribe_token
    ON status_subscribers (unsubscribe_token);

-- +goose Down
DROP TABLE status_subscribers;
