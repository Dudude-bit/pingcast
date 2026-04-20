-- +goose Up
-- Add manual-incident machinery (spec: status-page pivot §6 Sprint 1).
-- Existing auto-detected incidents keep flowing through Create/Resolve
-- on the worker side and default to state='investigating' → 'resolved';
-- manual incidents are created by users through the Pro-gated API.
CREATE TYPE incident_state AS ENUM (
    'investigating',
    'identified',
    'monitoring',
    'resolved'
);

ALTER TABLE incidents
    ADD COLUMN state incident_state NOT NULL DEFAULT 'investigating',
    ADD COLUMN is_manual BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN title TEXT;

-- Backfill: any resolved row (resolved_at IS NOT NULL) gets state='resolved'.
UPDATE incidents SET state = 'resolved' WHERE resolved_at IS NOT NULL;

-- +goose Down
ALTER TABLE incidents DROP COLUMN state, DROP COLUMN is_manual, DROP COLUMN title;
DROP TYPE incident_state;
