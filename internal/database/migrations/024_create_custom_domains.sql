-- +goose Up
-- Pro-tier custom domain for public status pages. Users point
-- status.<their-domain>.com at our edge via CNAME; we validate the
-- CNAME, request a cert, and route requests with that Host header to
-- the status-page handler for their slug.
CREATE TYPE custom_domain_status AS ENUM (
    'pending',      -- row created, nothing validated
    'validated',    -- DNS probe succeeded, awaiting cert
    'active',       -- cert issued, routing live
    'failed'        -- validation or cert provisioning failed (see last_error)
);

CREATE TABLE custom_domains (
    id BIGSERIAL PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    hostname TEXT NOT NULL UNIQUE,
    -- validation_token echoes back in the .well-known probe during
    -- DNS validation, so a hostname that only CNAMEs to our edge
    -- (without proving ownership of the repo behind it) can't claim
    -- the domain.
    validation_token TEXT NOT NULL,
    status custom_domain_status NOT NULL DEFAULT 'pending',
    last_error TEXT,
    dns_validated_at TIMESTAMPTZ,
    cert_issued_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_custom_domains_user_id ON custom_domains (user_id);
CREATE INDEX idx_custom_domains_status ON custom_domains (status);

-- +goose Down
DROP TABLE custom_domains;
DROP TYPE custom_domain_status;
