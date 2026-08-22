-- Two index fixes found by auditing pg_stat_user_indexes against the queries
-- that actually exist in this repo.
--
-- pg_stat_database.stats_reset is NULL here, so every idx_scan is a lifetime
-- count. That made two large indexes look dead (session_answer_correct_answered_idx,
-- 1976 kB, and event_batch_created_idx, 1880 kB, both on hot write paths).
-- Neither is dropped, and the reason is worth writing down so the next audit
-- does not rediscover them and reach the faster conclusion:
--
--   session_answer_correct_answered_idx backs leaderboard.RebuildPeriod's
--   GROUP BY over answered_at with is_correct (queries/leaderboard.sql). Zero
--   scans means that rebuild has not run yet, not that nothing needs it -- and
--   without it that aggregation is a full scan of the fastest-growing table in
--   the schema.
--
--   event_batch_created_idx has no reader at all today: the table is written by
--   events.Service and read only through its primary key (profile_id,
--   idempotency_key), including the account-deletion DELETE. But it is exactly
--   the index a retention sweep would need, and event_batch currently grows
--   without bound to hold idempotency keys forever. The bug there is the
--   missing sweep, not the index waiting for it.
--
-- An index whose reader is dormant is not a dead index.

-- ---------------------------------------------------------------------------
-- 1. Give the broadcast reclaim sweep an index it can use.
-- ---------------------------------------------------------------------------
-- broadcast_recipient_claim_idx is partial on status IN ('pending','failed'),
-- which is correct for claiming work. But reclaimStaleProcessing
-- (internal/broadcast/service.go) runs every 2 seconds looking for
-- status = 'processing' rows whose lease expired, and 'processing' falls
-- outside that partial index -- so the sweep seq-scans on every single tick,
-- forever, whether or not any campaign is running.
--
-- Partial on the same principle as the claim index: a row is 'processing' only
-- for the length of one delivery attempt, so this index stays near-empty no
-- matter how many campaigns have been sent.
CREATE INDEX broadcast_recipient_stale_idx
  ON broadcast_recipient (updated_at)
  WHERE status = 'processing';

-- ---------------------------------------------------------------------------
-- 2. Index the foreign keys whose parent rows actually get deleted.
-- ---------------------------------------------------------------------------
-- Postgres indexes the parent side of a foreign key automatically and the child
-- side never. Deleting a parent then scans the child table once per row removed.
--
-- Deleting a profile is a real user-facing operation (account deletion), and
-- broadcast_recipient gains one row per recipient per campaign -- one campaign
-- to a 100k-user base is 100k rows. Today that scan is free; at that size it is
-- a full table scan per account deleted.
CREATE INDEX broadcast_recipient_profile_idx ON broadcast_recipient (profile_id);
CREATE INDEX broadcast_recipient_notification_idx ON broadcast_recipient (notification_id);

-- Same reasoning on smaller tables that still grow with usage, each on the
-- cascade path of a profile or question delete.
CREATE INDEX support_message_sender_profile_idx ON support_message (sender_profile_id);
CREATE INDEX b2b_station_profile_idx ON b2b_station (station_profile_id);
CREATE INDEX telegram_quiz_poll_question_idx ON telegram_quiz_poll (question_id);

-- Left unindexed on purpose: payment_void.requested_by,
-- broadcast_campaign.created_by_admin and admin_audit_log_archive.admin_user_id
-- all point at admin_user. Deleting an admin is a rare manual act against tables
-- that stay in the low thousands, where the scan costs milliseconds -- and an
-- index with no reader is the thing this migration's header argues against
-- adding on speculation.
