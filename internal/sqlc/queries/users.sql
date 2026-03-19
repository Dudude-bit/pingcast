-- name: CreateUser :one
INSERT INTO users (email, slug, password_hash)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: GetUserBySlug :one
SELECT * FROM users WHERE slug = $1;

-- name: UpdateUserPlan :exec
UPDATE users SET plan = $2 WHERE id = $1;

-- name: UpdateUserTelegramChatID :exec
UPDATE users SET tg_chat_id = $2 WHERE id = $1;

-- name: UpdateUserLemonSqueezy :exec
UPDATE users
SET lemon_squeezy_customer_id = $2, lemon_squeezy_subscription_id = $3
WHERE id = $1;
