-- name: InsertBroadcastCampaign :one
INSERT INTO broadcast_campaign (
  created_by_admin, title, body, image_url, action_url,
  audience, channels, status, idempotency_key, queued_at
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, $9, now()
)
RETURNING *;

-- name: GetBroadcastCampaignByID :one
SELECT *
FROM broadcast_campaign
WHERE id = $1;

-- name: GetBroadcastCampaignByIdempotencyKey :one
SELECT *
FROM broadcast_campaign
WHERE idempotency_key = $1;

-- name: ListBroadcastCampaigns :many
SELECT *
FROM broadcast_campaign
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountBroadcastCampaigns :one
SELECT count(*)::int AS count
FROM broadcast_campaign;

-- name: CancelBroadcastCampaign :one
UPDATE broadcast_campaign
SET status = 'cancelled',
    finished_at = COALESCE(finished_at, now())
WHERE id = $1
  AND status IN ('queued', 'expanding', 'sending')
RETURNING *;

-- name: RetractBroadcastCampaign :one
-- Marks campaign cancelled after inbox rows are deleted by the service.
-- Already-delivered OS web-push cannot be recalled.
UPDATE broadcast_campaign
SET status = 'cancelled',
    error_summary = 'retracted: in-app notifications removed',
    finished_at = COALESCE(finished_at, now())
WHERE id = $1
  AND status IN ('queued', 'expanding', 'sending', 'completed', 'completed_with_errors', 'failed', 'cancelled')
RETURNING *;

-- name: DeleteInappNotificationsByCampaign :execrows
DELETE FROM notification
WHERE campaign_id = $1
  AND channel = 'inapp';

-- name: FailPendingRecipientsForCampaign :execrows
UPDATE broadcast_recipient
SET status = 'failed',
    last_error = 'campaign retracted',
    updated_at = now(),
    processed_at = COALESCE(processed_at, now())
WHERE campaign_id = $1
  AND status IN ('pending', 'processing', 'failed');

-- name: InsertInappNotification :one
INSERT INTO notification (profile_id, kind, payload, channel, campaign_id, sent_at)
VALUES ($1, $2, $3, 'inapp', $4, now())
RETURNING *;

-- name: GetInappNotificationByCampaignProfile :one
SELECT *
FROM notification
WHERE campaign_id = $1
  AND profile_id = $2
  AND channel = 'inapp'
LIMIT 1;

-- name: CountUnreadInappNotifications :one
SELECT count(*)::int AS count
FROM notification
WHERE profile_id = $1
  AND channel = 'inapp'
  AND read_at IS NULL;

-- name: ListInappNotifications :many
SELECT *
FROM notification
WHERE profile_id = sqlc.arg(profile_id)
  AND channel = 'inapp'
  AND (sqlc.narg('before')::timestamptz IS NULL OR created_at < sqlc.narg('before'))
ORDER BY created_at DESC
LIMIT sqlc.arg('limit');

-- name: MarkInappNotificationRead :one
UPDATE notification
SET read_at = COALESCE(read_at, now())
WHERE id = $1
  AND profile_id = $2
  AND channel = 'inapp'
RETURNING *;

-- name: MarkAllInappNotificationsRead :execrows
UPDATE notification
SET read_at = now()
WHERE profile_id = $1
  AND channel = 'inapp'
  AND read_at IS NULL;
