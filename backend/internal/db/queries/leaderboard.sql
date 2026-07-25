-- name: CountCorrectAnswersByProfileInRange :many
-- Used by leaderboard.Service.RebuildPeriod to recompute a Redis sorted
-- set from the durable session_answer table. from_ts is inclusive, to_ts
-- is exclusive.
SELECT
  es.profile_id,
  count(*)::int AS correct_count,
  max(sa.answered_at)::timestamptz AS last_answered_at
FROM session_answer sa
JOIN exam_session es ON es.id = sa.session_id
WHERE sa.is_correct
  AND sa.answered_at >= sqlc.arg(from_ts)
  AND sa.answered_at < sqlc.arg(to_ts)
GROUP BY es.profile_id;

-- name: CountCorrectAnswersByProfileByDayInRange :many
-- Per-profile, per-day correct-answer counts within [from_ts, to_ts) —
-- used by RebuildPeriod to reapply the daily point cap when reconstructing
-- a period's leaderboard from Postgres (the single per-profile total from
-- CountCorrectAnswersByProfileInRange can't distinguish "30 in one day"
-- from "10 a day for 3 days," and only the former should ever be capped).
SELECT
  es.profile_id,
  date_trunc('day', sa.answered_at AT TIME ZONE 'UTC')::timestamptz AS day,
  count(*)::int AS correct_count,
  max(sa.answered_at)::timestamptz AS last_answered_at
FROM session_answer sa
JOIN exam_session es ON es.id = sa.session_id
WHERE sa.is_correct
  AND sa.answered_at >= sqlc.arg(from_ts)
  AND sa.answered_at < sqlc.arg(to_ts)
GROUP BY es.profile_id, date_trunc('day', sa.answered_at AT TIME ZONE 'UTC');

-- name: ListProfileNamesByIDs :many
-- Batch name resolution for leaderboard display (top-N + around-you +
-- the requesting profile). Never exposes phone numbers.
SELECT id, name FROM profile WHERE id = ANY(sqlc.arg(ids)::uuid[]);
