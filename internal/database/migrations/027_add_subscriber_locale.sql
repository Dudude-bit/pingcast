-- +goose Up
-- Per-subscriber locale for outbound emails. Saved when a visitor
-- subscribes from a localised page; consumed by NotifyIncident when
-- fanning out updates so each subscriber gets their language.
-- NULL = no preference recorded → default to "en" in the app layer.
ALTER TABLE status_subscribers ADD COLUMN locale TEXT;
ALTER TABLE blog_subscribers ADD COLUMN locale TEXT;

-- +goose Down
ALTER TABLE status_subscribers DROP COLUMN locale;
ALTER TABLE blog_subscribers DROP COLUMN locale;
