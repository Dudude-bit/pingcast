-- name: UpsertCustomDomainCert :exec
INSERT INTO custom_domain_certs (custom_domain_id, cert_pem, key_pem, chain_pem, expires_at)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (custom_domain_id) DO UPDATE
SET cert_pem = EXCLUDED.cert_pem,
    key_pem = EXCLUDED.key_pem,
    chain_pem = EXCLUDED.chain_pem,
    expires_at = EXCLUDED.expires_at,
    issued_at = NOW();

-- name: GetCustomDomainCertByDomainID :one
SELECT id, custom_domain_id, cert_pem, key_pem, chain_pem, expires_at, issued_at
FROM custom_domain_certs
WHERE custom_domain_id = $1;

-- name: ListCustomDomainCertsExpiringBefore :many
SELECT id, custom_domain_id, cert_pem, key_pem, chain_pem, expires_at, issued_at
FROM custom_domain_certs
WHERE expires_at < $1;

-- name: ListExpiringHostnames :many
-- Renewal-loop helper: returns just the hostnames whose cert expires
-- before the given time AND whose custom_domain row is still active.
-- Inactive (deleted/failed) domains aren't worth renewing for.
SELECT d.hostname
FROM custom_domain_certs c
JOIN custom_domains d ON d.id = c.custom_domain_id
WHERE c.expires_at < $1
  AND d.status = 'active';
