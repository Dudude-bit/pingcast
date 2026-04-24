-- +goose Up
-- Newsletter subscribers for the /blog feed. Separate table from
-- status_subscribers because (a) there is no per-user slug scoping —
-- this is the PingCast newsletter, global; (b) we don't want to
-- accidentally mix product-incident emails with newsletter content.
-- Both use the same double-opt-in flow (confirm_token / unsubscribe_token).
CREATE TABLE blog_subscribers (
    id BIGSERIAL PRIMARY KEY,
    email TEXT NOT NULL UNIQUE,
    confirm_token TEXT NOT NULL,
    unsubscribe_token TEXT NOT NULL,
    confirmed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- Source tag so we can measure which signup placement converts
    -- (footer, /blog sidebar, individual post CTA). Nullable = "direct".
    source TEXT
);

CREATE UNIQUE INDEX idx_blog_subs_confirm_token
    ON blog_subscribers (confirm_token);
CREATE UNIQUE INDEX idx_blog_subs_unsubscribe_token
    ON blog_subscribers (unsubscribe_token);
CREATE INDEX idx_blog_subs_confirmed
    ON blog_subscribers (confirmed_at)
    WHERE confirmed_at IS NOT NULL;

-- +goose Down
DROP TABLE blog_subscribers;
