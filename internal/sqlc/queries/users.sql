-- name: CreateUser :one
INSERT INTO users (email, slug, password_hash)
VALUES ($1, $2, $3)
RETURNING id, email, slug, password_hash, plan, lemon_squeezy_customer_id, lemon_squeezy_subscription_id, created_at, deleted_at;

-- name: GetUserByID :one
SELECT id, email, slug, plan, lemon_squeezy_customer_id, lemon_squeezy_subscription_id, created_at, deleted_at
FROM users WHERE id = $1 AND deleted_at IS NULL;

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
