-- name: UpsertPushSubscription :one
INSERT INTO push_subscription (profile_id, endpoint, p256dh, auth, user_agent)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (endpoint) DO UPDATE SET
  profile_id = EXCLUDED.profile_id,
  p256dh = EXCLUDED.p256dh,
  auth = EXCLUDED.auth,
  user_agent = EXCLUDED.user_agent,
  last_seen = now()
RETURNING *;

-- name: DeletePushSubscription :execrows
DELETE FROM push_subscription
WHERE profile_id = $1 AND endpoint = $2;

-- name: CountPushSubscriptions :one
SELECT count(*)::int AS count
FROM push_subscription
WHERE profile_id = $1;

-- name: ListPushSubscriptions :many
SELECT *
FROM push_subscription
WHERE profile_id = $1
ORDER BY last_seen DESC;

-- name: DeletePushSubscriptionByEndpoint :execrows
DELETE FROM push_subscription
WHERE endpoint = $1;

-- name: InsertNotification :one
INSERT INTO notification (profile_id, kind, payload, channel)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: MarkNotificationSent :exec
UPDATE notification
SET sent_at = now()
WHERE id = $1;
