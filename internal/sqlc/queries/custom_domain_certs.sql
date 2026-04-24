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
