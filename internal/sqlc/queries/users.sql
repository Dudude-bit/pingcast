-- name: CreateUser :one
INSERT INTO users (email, slug, password_hash)
VALUES ($1, $2, $3)
RETURNING id, email, slug, password_hash, plan, lemon_squeezy_customer_id, lemon_squeezy_subscription_id, created_at, deleted_at;

-- name: GetUserByID :one
SELECT id, email, slug, plan, lemon_squeezy_customer_id, lemon_squeezy_subscription_id, created_at, deleted_at
FROM users WHERE id = $1 AND deleted_at IS NULL;

-- name: GetUserBranding :one
SELECT logo_url, accent_color, custom_footer_text
FROM users WHERE id = $1 AND deleted_at IS NULL;

-- name: GetUserBrandingBySlug :one
SELECT u.plan, u.logo_url, u.accent_color, u.custom_footer_text
FROM users u
WHERE u.slug = $1 AND u.deleted_at IS NULL;

-- name: UpdateUserBranding :exec
UPDATE users
SET logo_url = $2, accent_color = $3, custom_footer_text = $4
WHERE id = $1 AND deleted_at IS NULL;

-- name: GetUserByEmail :one
SELECT id, email, slug, password_hash, plan, created_at, deleted_at
FROM users WHERE email = $1 AND deleted_at IS NULL;

-- name: GetUserBySlug :one
SELECT id, email, slug, plan, created_at, deleted_at
FROM users WHERE slug = $1 AND deleted_at IS NULL;

-- name: UpdateUserPlan :exec
UPDATE users SET plan = $2 WHERE id = $1 AND deleted_at IS NULL;

-- name: UpdateUserLemonSqueezy :exec
UPDATE users
SET lemon_squeezy_customer_id = $2, lemon_squeezy_subscription_id = $3
WHERE id = $1 AND deleted_at IS NULL;

-- name: SetSubscriptionVariant :exec
UPDATE users SET subscription_variant = $2 WHERE id = $1 AND deleted_at IS NULL;

-- name: CountActiveFounderSubscriptions :one
SELECT COUNT(*)::bigint AS count
FROM users
WHERE plan = 'pro'
  AND subscription_variant = 'founder'
  AND deleted_at IS NULL;
