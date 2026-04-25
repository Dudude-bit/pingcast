-- name: CreateBlogSubscriber :one
INSERT INTO blog_subscribers (email, confirm_token, unsubscribe_token, source, locale)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, email, confirm_token, unsubscribe_token, confirmed_at, created_at, source, locale;

-- name: ConfirmBlogSubscriber :one
UPDATE blog_subscribers
SET confirmed_at = NOW()
WHERE confirm_token = $1 AND confirmed_at IS NULL
RETURNING id, email, confirm_token, unsubscribe_token, confirmed_at, created_at, source, locale;

-- name: UnsubscribeBlogSubscriber :one
DELETE FROM blog_subscribers
WHERE unsubscribe_token = $1
RETURNING id, email, confirm_token, unsubscribe_token, confirmed_at, created_at, source, locale;

-- name: ListConfirmedBlogSubscribers :many
SELECT id, email, confirm_token, unsubscribe_token, confirmed_at, created_at, source, locale
FROM blog_subscribers
WHERE confirmed_at IS NOT NULL
ORDER BY created_at DESC;

-- name: CountConfirmedBlogSubscribers :one
SELECT COUNT(*)::bigint FROM blog_subscribers WHERE confirmed_at IS NOT NULL;
