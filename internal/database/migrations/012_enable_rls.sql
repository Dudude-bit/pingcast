-- Row-Level Security for multi-tenancy defense-in-depth.
-- App sets: SET LOCAL app.current_user_id = '<uuid>' at the start of each transaction.
-- System queries (scheduler, cleanup) use a role that bypasses RLS.

ALTER TABLE monitors ENABLE ROW LEVEL SECURITY;
ALTER TABLE notification_channels ENABLE ROW LEVEL SECURITY;
ALTER TABLE check_results ENABLE ROW LEVEL SECURITY;
ALTER TABLE incidents ENABLE ROW LEVEL SECURITY;

-- Policies: only return rows belonging to the current user
CREATE POLICY monitors_user_isolation ON monitors
    USING (user_id = current_setting('app.current_user_id', true)::uuid);

CREATE POLICY channels_user_isolation ON notification_channels
    USING (user_id = current_setting('app.current_user_id', true)::uuid);

-- check_results and incidents are linked to monitors, not directly to users.
-- Join through monitors for user isolation.
CREATE POLICY check_results_user_isolation ON check_results
    USING (monitor_id IN (
        SELECT id FROM monitors
        WHERE user_id = current_setting('app.current_user_id', true)::uuid
    ));

CREATE POLICY incidents_user_isolation ON incidents
    USING (monitor_id IN (
        SELECT id FROM monitors
        WHERE user_id = current_setting('app.current_user_id', true)::uuid
    ));

-- The application's DB user (pingcast) is the table owner, so RLS doesn't
-- apply by default. Force it with FORCE ROW LEVEL SECURITY.
ALTER TABLE monitors FORCE ROW LEVEL SECURITY;
ALTER TABLE notification_channels FORCE ROW LEVEL SECURITY;
ALTER TABLE check_results FORCE ROW LEVEL SECURITY;
ALTER TABLE incidents FORCE ROW LEVEL SECURITY;
