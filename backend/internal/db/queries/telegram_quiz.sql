-- name: UpsertTelegramChat :exec
INSERT INTO telegram_chat (chat_id, title, chat_type, bot_status, updated_at)
VALUES ($1, $2, $3, $4, now())
ON CONFLICT (chat_id) DO UPDATE SET
  title = EXCLUDED.title,
  chat_type = EXCLUDED.chat_type,
  bot_status = EXCLUDED.bot_status,
  updated_at = now();

-- name: GetActiveQuizSessionByChat :one
SELECT id, chat_id, started_by_tg_user_id, active, question_id, awaiting_answer,
       answer_message_id, asked_count, correct_count, last_activity_at, created_at
FROM telegram_quiz_session
WHERE chat_id = $1 AND active = true;

-- name: GetActiveQuizSessionByChatForUpdate :one
SELECT id, chat_id, started_by_tg_user_id, active, question_id, awaiting_answer,
       answer_message_id, asked_count, correct_count, last_activity_at, created_at
FROM telegram_quiz_session
WHERE chat_id = $1 AND active = true
FOR UPDATE;

-- name: CreateQuizSession :one
INSERT INTO telegram_quiz_session (chat_id, started_by_tg_user_id)
VALUES ($1, $2)
RETURNING id, chat_id, started_by_tg_user_id, active, question_id, awaiting_answer,
          answer_message_id, asked_count, correct_count, last_activity_at, created_at;

-- name: DeactivateQuizSession :exec
UPDATE telegram_quiz_session
SET active = false, awaiting_answer = false, last_activity_at = now()
WHERE id = $1 AND active = true;

-- name: DeactivateActiveQuizSessionsForChat :exec
UPDATE telegram_quiz_session
SET active = false, awaiting_answer = false, last_activity_at = now()
WHERE chat_id = $1 AND active = true;

-- name: SetQuizSessionQuestion :exec
UPDATE telegram_quiz_session
SET question_id = $2,
    awaiting_answer = true,
    answer_message_id = $3,
    asked_count = asked_count + 1,
    last_activity_at = now()
WHERE id = $1 AND active = true;

-- name: MarkQuizSessionAnswered :execrows
UPDATE telegram_quiz_session
SET awaiting_answer = false,
    correct_count = correct_count + sqlc.arg(correct_delta),
    last_activity_at = now()
WHERE id = sqlc.arg(id) AND active = true AND awaiting_answer = true;

-- name: TouchQuizSessionActivity :exec
UPDATE telegram_quiz_session
SET last_activity_at = now()
WHERE id = $1;

-- name: ListQuizAnswers :many
-- Includes is_correct for bot grading (never exposed on the public content API).
SELECT a.id, a.position, a.is_correct,
       COALESCE(at.text, aft.text, '') AS text
FROM answer a
LEFT JOIN answer_translation at
       ON at.answer_id = a.id AND at.locale = sqlc.arg(locale) AND at.status = 'verified'
LEFT JOIN answer_translation aft
       ON aft.answer_id = a.id AND aft.locale = 'uz-Latn' AND aft.status = 'verified'
WHERE a.question_id = sqlc.arg(question_id)
ORDER BY a.position;

-- name: DeleteTelegramAccountByTgUserID :execrows
DELETE FROM telegram_account WHERE tg_user_id = $1;

-- name: ListTelegramDigestCandidates :many
-- Linked profiles with ≥1 due FSRS card, excluding recent telegram digests.
SELECT ta.tg_user_id, p.id AS profile_id, p.locale_pref,
       COUNT(qm.question_id)::int AS due_count
FROM telegram_account ta
JOIN profile p ON p.id = ta.profile_id AND p.status = 'active'
JOIN question_memory qm ON qm.profile_id = p.id AND qm.due_at <= now()
JOIN question q ON q.id = qm.question_id AND q.validation_status = 'valid'
WHERE NOT EXISTS (
  SELECT 1 FROM notification n
  WHERE n.profile_id = p.id
    AND n.kind = sqlc.arg(kind)
    AND n.channel = 'telegram'
    AND n.created_at > now() - (sqlc.arg(cooldown)::text)::interval
)
GROUP BY ta.tg_user_id, p.id, p.locale_pref
HAVING COUNT(qm.question_id) > 0
ORDER BY due_count DESC, p.id
LIMIT sqlc.arg(limit_count);
