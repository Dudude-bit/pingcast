-- +goose Up
-- Pro-tier status-page branding. Columns live on users (one status page
-- per user via the slug) — if we ever split to multiple status pages
-- per account, move to a dedicated status_page_branding table.
ALTER TABLE users
    ADD COLUMN logo_url TEXT,
    ADD COLUMN accent_color TEXT,
    ADD COLUMN custom_footer_text TEXT;

-- +goose Down
ALTER TABLE users
    DROP COLUMN logo_url,
    DROP COLUMN accent_color,
    DROP COLUMN custom_footer_text;
