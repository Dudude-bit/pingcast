-- +goose Up
-- Manual status updates attached to an incident. A Pro user posts one
-- each time they want to announce a state transition ("we've identified
-- the problem", "monitoring the fix", etc.); auto-detected incidents
-- don't use this table.
CREATE TABLE incident_updates (
    id BIGSERIAL PRIMARY KEY,
    incident_id BIGINT NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    state incident_state NOT NULL,
    body TEXT NOT NULL,
    posted_by_user_id UUID NOT NULL REFERENCES users(id),
    posted_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_incident_updates_incident_id_posted_at
    ON incident_updates (incident_id, posted_at DESC);

-- +goose Down
DROP TABLE incident_updates;
