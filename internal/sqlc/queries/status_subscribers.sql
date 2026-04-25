-- name: CreateStatusSubscriber :one
INSERT INTO status_subscribers (slug, email, confirm_token, unsubscribe_token, locale)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, slug, email, confirm_token, unsubscribe_token, confirmed_at, created_at, locale;

-- name: ConfirmStatusSubscriber :one
UPDATE status_subscribers
SET confirmed_at = NOW()
WHERE confirm_token = $1 AND confirmed_at IS NULL
RETURNING id, slug, email, confirm_token, unsubscribe_token, confirmed_at, created_at, locale;

-- name: UnsubscribeStatusSubscriber :one
DELETE FROM status_subscribers
WHERE unsubscribe_token = $1
RETURNING id, slug, email, confirm_token, unsubscribe_token, confirmed_at, created_at, locale;

-- name: ListConfirmedSubscribersBySlug :many
SELECT id, slug, email, confirm_token, unsubscribe_token, confirmed_at, created_at, locale
FROM status_subscribers
WHERE slug = $1 AND confirmed_at IS NOT NULL
ORDER BY created_at;
