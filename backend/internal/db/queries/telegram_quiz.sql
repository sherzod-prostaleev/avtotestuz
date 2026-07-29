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
       answer_message_id, asked_count, correct_count, last_activity_at, created_at,
       total_questions, question_no, mode
FROM telegram_quiz_session
WHERE chat_id = $1 AND active = true;

-- name: GetActiveQuizSessionByChatForUpdate :one
SELECT id, chat_id, started_by_tg_user_id, active, question_id, awaiting_answer,
       answer_message_id, asked_count, correct_count, last_activity_at, created_at,
       total_questions, question_no, mode
FROM telegram_quiz_session
WHERE chat_id = $1 AND active = true
FOR UPDATE;

-- name: CreateQuizSession :one
INSERT INTO telegram_quiz_session (chat_id, started_by_tg_user_id)
VALUES ($1, $2)
RETURNING id, chat_id, started_by_tg_user_id, active, question_id, awaiting_answer,
          answer_message_id, asked_count, correct_count, last_activity_at, created_at,
          total_questions, question_no, mode;

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

-- name: SetQuizSessionMode :exec
UPDATE telegram_quiz_session
SET mode = sqlc.arg(mode), total_questions = sqlc.arg(total_questions)
WHERE id = sqlc.arg(id);

-- name: AdvanceQuizSessionQuestion :one
UPDATE telegram_quiz_session
SET question_no = question_no + 1, last_activity_at = now()
WHERE id = $1 AND active = true
RETURNING question_no;

-- name: CreateQuizPoll :exec
INSERT INTO telegram_quiz_poll
  (poll_id, session_id, question_id, question_no, correct_idx)
VALUES ($1, $2, $3, $4, $5);

-- name: GetQuizPoll :one
SELECT poll_id, session_id, question_id, question_no, correct_idx, opened_at, closed
FROM telegram_quiz_poll WHERE poll_id = $1;

-- name: CloseQuizPoll :exec
UPDATE telegram_quiz_poll SET closed = true WHERE poll_id = $1;

-- name: UpsertQuizParticipant :exec
INSERT INTO telegram_quiz_participant
  (session_id, tg_user_id, display_name, answered_count, correct_count, total_ms)
VALUES (
  sqlc.arg(session_id), sqlc.arg(tg_user_id), sqlc.arg(display_name),
  1, sqlc.arg(correct_delta), sqlc.arg(elapsed_ms)
)
ON CONFLICT (session_id, tg_user_id) DO UPDATE SET
  answered_count = telegram_quiz_participant.answered_count + 1,
  correct_count  = telegram_quiz_participant.correct_count + EXCLUDED.correct_count,
  total_ms       = telegram_quiz_participant.total_ms + EXCLUDED.total_ms,
  display_name   = CASE WHEN EXCLUDED.display_name <> ''
                        THEN EXCLUDED.display_name
                        ELSE telegram_quiz_participant.display_name END;

-- name: ListQuizRanking :many
-- Reyting: to'g'ri javob soni, tenglikda o'rtacha javob vaqti tezrog'i.
SELECT tg_user_id, display_name, answered_count, correct_count, total_ms
FROM telegram_quiz_participant
WHERE session_id = $1
ORDER BY correct_count DESC,
         (total_ms::numeric / GREATEST(answered_count, 1)) ASC,
         first_seen_at ASC;

-- name: RandomPollableQuestionIDs :many
-- Telegram poll varianti 100 belgidan oshmasligi kerak: uzun javobli
-- savollar tanlanmaydi (kesish o'rniga chetlab o'tiladi).
SELECT q.id FROM question q
WHERE q.validation_status = 'valid'
  AND (q.image_id IS NOT NULL) = sqlc.arg(has_image)::boolean
  AND (SELECT COUNT(*) FROM answer a WHERE a.question_id = q.id) BETWEEN 2 AND 10
  AND NOT EXISTS (
    SELECT 1 FROM answer a
    JOIN answer_translation at
      ON at.answer_id = a.id AND at.locale = 'uz-Latn' AND at.status = 'verified'
    WHERE a.question_id = q.id
      AND char_length(at.text) > sqlc.arg(max_answer_len)::int
  )
ORDER BY random()
LIMIT sqlc.arg(limit_count);

-- name: GetLimitConfigValue :one
SELECT free_value FROM limit_config WHERE key = $1;
