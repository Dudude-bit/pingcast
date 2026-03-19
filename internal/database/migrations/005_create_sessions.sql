CREATE TABLE sessions (
    id VARCHAR(64) PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_sessions_expires_at ON sessions (expires_at);
CREATE INDEX idx_sessions_user_id ON sessions (user_id);
