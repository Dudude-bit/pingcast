-- name: CreateCustomDomain :one
INSERT INTO custom_domains (user_id, hostname, validation_token)
VALUES ($1, $2, $3)
RETURNING id, user_id, hostname, validation_token, status, last_error,
          dns_validated_at, cert_issued_at, created_at;

-- name: ListCustomDomainsByUserID :many
SELECT id, user_id, hostname, validation_token, status, last_error,
       dns_validated_at, cert_issued_at, created_at
FROM custom_domains
WHERE user_id = $1
ORDER BY created_at DESC;

-- name: GetCustomDomainByHostname :one
SELECT id, user_id, hostname, validation_token, status, last_error,
       dns_validated_at, cert_issued_at, created_at
FROM custom_domains
WHERE hostname = $1;

-- name: ListPendingCustomDomains :many
-- Driven by the DNS-validation worker. Returns everything not yet
-- active so the worker retries failures on the next tick.
SELECT id, user_id, hostname, validation_token, status, last_error,
       dns_validated_at, cert_issued_at, created_at
FROM custom_domains
WHERE status IN ('pending', 'validated', 'failed')
ORDER BY created_at ASC
LIMIT 100;

-- name: UpdateCustomDomainStatus :exec
UPDATE custom_domains
SET status = $2, last_error = $3,
    dns_validated_at = COALESCE($4, dns_validated_at),
    cert_issued_at = COALESCE($5, cert_issued_at)
WHERE id = $1;

-- name: DeleteCustomDomain :exec
DELETE FROM custom_domains WHERE id = $1 AND user_id = $2;

-- name: ListActiveCustomDomainHostnames :many
-- Consumed by the host-header lookup cache. Only active domains route
-- real traffic.
SELECT hostname, user_id FROM custom_domains WHERE status = 'active';
