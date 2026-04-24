-- +goose Up
-- TLS material per custom domain. Issued by ACME (HTTP-01 by default;
-- DNS-01 if a per-tenant DNS provider becomes plug-able later). Stored
-- here so an external Traefik / Caddy file-provider can pick it up
-- without round-tripping through ACME again on each ingress restart.
CREATE TABLE custom_domain_certs (
    id BIGSERIAL PRIMARY KEY,
    custom_domain_id BIGINT NOT NULL REFERENCES custom_domains(id) ON DELETE CASCADE,
    -- Storing PEM blobs is simpler than splitting into x.509 / private-key
    -- columns. Renewals overwrite the row in-place.
    cert_pem TEXT NOT NULL,
    key_pem TEXT NOT NULL,
    -- ACME chain (intermediate certs) — keeps the row self-contained.
    chain_pem TEXT NOT NULL DEFAULT '',
    expires_at TIMESTAMPTZ NOT NULL,
    issued_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (custom_domain_id)
);

-- Renewal loop scans for certs near expiry (default: ≤ 30 days out).
CREATE INDEX idx_custom_domain_certs_expires
    ON custom_domain_certs (expires_at);

-- +goose Down
DROP TABLE custom_domain_certs;
