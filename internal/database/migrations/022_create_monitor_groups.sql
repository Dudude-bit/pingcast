-- +goose Up
-- Monitor groups: collapsible sections on the public status page so a
-- SaaS with 30 monitors doesn't show a 30-row flat list. One group
-- belongs to one user; one monitor optionally belongs to one group.
CREATE TABLE monitor_groups (
    id BIGSERIAL PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    ordering INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_monitor_groups_user_id ON monitor_groups (user_id, ordering);

ALTER TABLE monitors ADD COLUMN group_id BIGINT REFERENCES monitor_groups(id) ON DELETE SET NULL;
CREATE INDEX idx_monitors_group_id ON monitors (group_id) WHERE group_id IS NOT NULL;

-- +goose Down
ALTER TABLE monitors DROP COLUMN group_id;
DROP TABLE monitor_groups;
