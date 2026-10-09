-- name: UpsertTelegramBotUser :exec
-- Every private-chat update. Any update from the user proves the chat is
-- open again, so blocked_at is cleared. The WHERE skips the write when
-- nothing changed and the row was touched in the last minute, so a busy
-- chat does not turn into one row rewrite per message.
INSERT INTO telegram_bot_user (tg_user_id, first_name, username, language_code)
VALUES (sqlc.arg(tg_user_id), sqlc.arg(first_name), sqlc.arg(username), sqlc.arg(language_code))
ON CONFLICT (tg_user_id) DO UPDATE SET
  first_name    = EXCLUDED.first_name,
  username      = EXCLUDED.username,
  language_code = EXCLUDED.language_code,
  last_seen_at  = now(),
  blocked_at    = NULL
WHERE telegram_bot_user.blocked_at IS NOT NULL
   OR telegram_bot_user.last_seen_at < now() - interval '1 minute'
   OR telegram_bot_user.first_name    IS DISTINCT FROM EXCLUDED.first_name
   OR telegram_bot_user.username      IS DISTINCT FROM EXCLUDED.username
   OR telegram_bot_user.language_code IS DISTINCT FROM EXCLUDED.language_code;

-- name: UpsertTelegramBotUserFromWebApp :exec
-- A Mini App sign-in or link whose signed launch data says the user allows
-- the bot to message them (allows_write_to_pm): they join the audience even
-- if they never typed in the chat. Unlike UpsertTelegramBotUser it leaves
-- blocked_at alone — launching the Mini App does not prove the user
-- unblocked the bot; my_chat_member or a private message does. Same
-- once-a-minute write guard.
INSERT INTO telegram_bot_user (tg_user_id, first_name, username, language_code)
VALUES (sqlc.arg(tg_user_id), sqlc.arg(first_name), sqlc.arg(username), sqlc.arg(language_code))
ON CONFLICT (tg_user_id) DO UPDATE SET
  first_name    = EXCLUDED.first_name,
  username      = EXCLUDED.username,
  language_code = EXCLUDED.language_code,
  last_seen_at  = now()
WHERE telegram_bot_user.last_seen_at < now() - interval '1 minute'
   OR telegram_bot_user.first_name    IS DISTINCT FROM EXCLUDED.first_name
   OR telegram_bot_user.username      IS DISTINCT FROM EXCLUDED.username
   OR telegram_bot_user.language_code IS DISTINCT FROM EXCLUDED.language_code;

-- name: MarkTelegramBotUserBlocked :exec
-- my_chat_member kicked in the private chat, or a 403 on send.
INSERT INTO telegram_bot_user (tg_user_id, blocked_at)
VALUES ($1, now())
ON CONFLICT (tg_user_id) DO UPDATE SET blocked_at = now();

-- name: ToggleTelegramReminders :one
-- /eslatma. A user without a row has the default (on), so toggling inserts off.
INSERT INTO telegram_bot_user (tg_user_id, reminders_enabled)
VALUES ($1, false)
ON CONFLICT (tg_user_id) DO UPDATE
  SET reminders_enabled = NOT telegram_bot_user.reminders_enabled
RETURNING reminders_enabled;

-- name: DisableTelegramReminders :exec
INSERT INTO telegram_bot_user (tg_user_id, reminders_enabled)
VALUES ($1, false)
ON CONFLICT (tg_user_id) DO UPDATE SET reminders_enabled = false;

-- name: GetTelegramBotUser :one
SELECT tg_user_id, first_name, username, language_code, first_seen_at, last_seen_at,
       reminders_enabled, blocked_at, last_reminder_on, last_signup_pitch_on
FROM telegram_bot_user WHERE tg_user_id = $1;

-- name: ClaimTelegramReminderRecipient :one
-- Claims one recipient for the day and records the claim in the same
-- statement: a crash after this never sends to them twice, and SKIP LOCKED
-- lets a second pass (another replica) take a different user. The order
-- matches telegram_bot_user_reminder_claim_idx (migration 0079): unclaimed
-- users come first in it, so one claim reads one index entry rather than
-- walking past everyone claimed earlier today.
UPDATE telegram_bot_user u
SET last_reminder_on = sqlc.arg(day)::date
WHERE u.tg_user_id = (
  SELECT c.tg_user_id FROM telegram_bot_user c
  WHERE c.reminders_enabled
    AND c.blocked_at IS NULL
    AND (c.last_reminder_on IS NULL OR c.last_reminder_on < sqlc.arg(day)::date)
  ORDER BY c.last_reminder_on NULLS FIRST, c.tg_user_id
  LIMIT 1
  FOR UPDATE SKIP LOCKED
)
RETURNING u.tg_user_id;

-- name: ReleaseTelegramReminderClaim :exec
-- Gives today's claim back when the send provably never left this host
-- (connect-phase failure), so the next tick tries the user again. NULL puts
-- them first in the claim order. Guarded on the day: a user claimed for a
-- later day is untouched.
UPDATE telegram_bot_user
SET last_reminder_on = NULL
WHERE tg_user_id = sqlc.arg(tg_user_id) AND last_reminder_on = sqlc.arg(day)::date;

-- name: CountTelegramReminderAudience :one
SELECT COUNT(*)::int AS total,
       COUNT(*) FILTER (WHERE NOT reminders_enabled)::int AS opted_out,
       COUNT(*) FILTER (WHERE reminders_enabled AND blocked_at IS NOT NULL)::int AS blocked,
       COUNT(*) FILTER (WHERE reminders_enabled AND blocked_at IS NULL)::int AS eligible,
       COUNT(*) FILTER (WHERE reminders_enabled AND blocked_at IS NULL
                          AND (last_reminder_on IS NULL OR last_reminder_on < sqlc.arg(day)::date))::int AS pending
FROM telegram_bot_user;

-- name: ListTelegramReminderAudience :many
-- Personal-line inputs for eligible users: one user (after a claim) or all
-- of them (dry-run). Profile numbers come only from an active profile; a
-- link to a banned profile counts as linked with nothing to report.
SELECT u.tg_user_id, u.first_name, u.language_code, u.last_signup_pitch_on,
       (ta.tg_user_id IS NOT NULL)::bool AS linked,
       (ta.phone_verified_at IS NOT NULL)::bool AS phone_verified,
       COALESCE(s.current, 0)::int AS streak_current,
       s.last_active_date AS last_active_date,
       COALESCE(due.n, 0)::int AS due_count,
       COALESCE(tickets.n, 0)::int AS tickets_completed
FROM telegram_bot_user u
LEFT JOIN telegram_account ta ON ta.tg_user_id = u.tg_user_id
LEFT JOIN profile p ON p.id = ta.profile_id AND p.status = 'active'
LEFT JOIN streak s ON s.profile_id = p.id
LEFT JOIN LATERAL (
  SELECT COUNT(*) AS n
  FROM question_memory qm
  JOIN question q ON q.id = qm.question_id AND q.validation_status = 'valid'
  WHERE qm.profile_id = p.id AND qm.due_at <= now()
) due ON p.id IS NOT NULL
LEFT JOIN LATERAL (
  SELECT COUNT(*) AS n
  FROM variant_progress vp
  WHERE vp.profile_id = p.id AND vp.completed_at IS NOT NULL
) tickets ON p.id IS NOT NULL
WHERE u.reminders_enabled AND u.blocked_at IS NULL
  AND (sqlc.narg(only_tg_user_id)::bigint IS NULL OR u.tg_user_id = sqlc.narg(only_tg_user_id)::bigint)
ORDER BY u.tg_user_id;

-- name: ListDailyQuestionCandidates :many
-- Questions that fit a Telegram quiz poll in both uz-Latn and ru (ru falls
-- back to uz-Latn like GetQuestion): question <= 300 chars, 2..10 answers of
-- <= 100 chars, exactly one correct. Never-used questions come first, then
-- those whose explanation fits the poll's 200-char limit in both languages,
-- then a per-day hash so the order is deterministic for a given day. Go
-- re-checks the winner with buildPollRequest before using it.
SELECT q.id, (ex.fits)::bool AS explanation_fits
FROM question q
JOIN question_translation qu
  ON qu.question_id = q.id AND qu.locale = 'uz-Latn' AND qu.status = 'verified'
LEFT JOIN question_translation qr
  ON qr.question_id = q.id AND qr.locale = 'ru' AND qr.status = 'verified'
CROSS JOIN LATERAL (
  SELECT COUNT(*)::int AS answer_count,
         COUNT(*) FILTER (WHERE a.is_correct)::int AS correct_count,
         COALESCE(BOOL_OR(
           au.text IS NULL OR btrim(au.text) = ''
           OR char_length(au.text) > sqlc.arg(max_option_len)::int
           OR char_length(COALESCE(ar.text, au.text)) > sqlc.arg(max_option_len)::int
         ), true) AS has_bad_answer
  FROM answer a
  LEFT JOIN answer_translation au ON au.answer_id = a.id AND au.locale = 'uz-Latn' AND au.status = 'verified'
  LEFT JOIN answer_translation ar ON ar.answer_id = a.id AND ar.locale = 'ru' AND ar.status = 'verified'
  WHERE a.question_id = q.id
) ans
CROSS JOIN LATERAL (
  SELECT COALESCE(bool_and(char_length(t.txt) <= sqlc.arg(max_explanation_len)::int), false) AS fits
  FROM (
    SELECT (SELECT string_agg(b->>'text', ' ') FROM jsonb_array_elements(COALESCE(et.blocks, eu.blocks)) b) AS txt
    FROM explanation e
    JOIN explanation_translation eu ON eu.explanation_id = e.id AND eu.locale = 'uz-Latn' AND eu.status = 'verified'
    LEFT JOIN explanation_translation et ON et.explanation_id = e.id AND et.locale = 'ru' AND et.status = 'verified'
    WHERE e.question_id = q.id
    UNION ALL
    SELECT (SELECT string_agg(b->>'text', ' ') FROM jsonb_array_elements(eu.blocks) b)
    FROM explanation e
    JOIN explanation_translation eu ON eu.explanation_id = e.id AND eu.locale = 'uz-Latn' AND eu.status = 'verified'
    WHERE e.question_id = q.id
  ) t
) ex
LEFT JOIN LATERAL (
  SELECT MAX(dq.day) AS last_day FROM telegram_daily_question dq
  WHERE dq.question_id = q.id AND dq.day <> sqlc.arg(day)::date
) used ON true
WHERE q.validation_status = 'valid'
  AND btrim(qu.text) <> ''
  AND char_length(qu.text) <= sqlc.arg(max_question_len)::int
  AND char_length(COALESCE(qr.text, qu.text)) <= sqlc.arg(max_question_len)::int
  AND ans.answer_count BETWEEN 2 AND sqlc.arg(max_options)::int
  AND ans.correct_count = 1
  AND NOT ans.has_bad_answer
ORDER BY used.last_day ASC NULLS FIRST, ex.fits DESC, md5(q.id::text || sqlc.arg(day)::date::text)
LIMIT sqlc.arg(limit_count);

-- name: InsertDailyQuestion :exec
INSERT INTO telegram_daily_question (day, question_id) VALUES ($1, $2)
ON CONFLICT (day) DO NOTHING;

-- name: ReplaceDailyQuestion :exec
-- The day's stored question stopped fitting a poll (edited mid-evening).
-- Conditional on the broken id, so replicas that both notice agree on one
-- replacement: the loser's UPDATE matches nothing and it re-reads the row.
UPDATE telegram_daily_question
SET question_id = sqlc.arg(new_question_id), created_at = now()
WHERE day = sqlc.arg(day) AND question_id = sqlc.arg(old_question_id);

-- name: MarkTelegramSignupPitch :exec
-- The signup pitch reached this user today; the next one waits 7 days.
UPDATE telegram_bot_user SET last_signup_pitch_on = sqlc.arg(day)::date
WHERE tg_user_id = sqlc.arg(tg_user_id);

-- name: GetDailyQuestion :one
SELECT question_id FROM telegram_daily_question WHERE day = $1;
