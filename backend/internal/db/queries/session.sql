-- name: ListVariantQuestionIDsOrdered :many
SELECT vq.question_id
FROM variant_question vq
JOIN question q ON q.id = vq.question_id AND q.validation_status = 'valid'
WHERE vq.variant_id = $1
ORDER BY vq.position;

-- name: RandomQuestionIDs :many
SELECT id FROM question
WHERE validation_status = 'valid'
ORDER BY random()
LIMIT $1;

-- name: RandomQuestionIDsByCategory :many
SELECT id FROM question
WHERE validation_status = 'valid' AND category_id = sqlc.arg(category_id)
ORDER BY random()
LIMIT sqlc.arg(limit_count);

-- name: RandomQuestionIDsBySign :many
SELECT q.id FROM question q
JOIN question_sign qs ON qs.question_id = q.id
WHERE q.validation_status = 'valid' AND qs.sign_id = sqlc.arg(sign_id)
ORDER BY random()
LIMIT sqlc.arg(limit_count);

-- name: RandomQuestionIDsByImagePresence :many
-- has_image=true selects illustrated questions, false selects text-only ones.
SELECT id FROM question
WHERE validation_status = 'valid'
  AND (image_id IS NOT NULL) = sqlc.arg(has_image)::boolean
ORDER BY random()
LIMIT sqlc.arg(limit_count);

-- name: ListMistakeBankQuestionIDs :many
SELECT qm.question_id
FROM question_memory qm
JOIN question q ON q.id = qm.question_id AND q.validation_status = 'valid'
WHERE qm.profile_id = sqlc.arg(profile_id) AND qm.lapses > 0 AND qm.due_at <= now()
ORDER BY qm.due_at ASC
LIMIT sqlc.arg(limit_count);

-- name: GetAnswerForScoring :one
-- Also validates that answer_id truly belongs to question_id.
SELECT id, question_id, is_correct FROM answer
WHERE id = sqlc.arg(id) AND question_id = sqlc.arg(question_id);

-- name: GetCorrectAnswerID :one
SELECT id FROM answer WHERE question_id = $1 AND is_correct = true;

-- name: GetVariantProgress :one
SELECT * FROM variant_progress
WHERE profile_id = sqlc.arg(profile_id) AND variant_id = sqlc.arg(variant_id);

-- name: GetCategoryIDByCode :one
SELECT id FROM category WHERE code = $1;

-- name: GetSignIDByCode :one
SELECT id FROM sign WHERE code = $1;

-- name: CreateExamSession :one
WITH created AS (
  INSERT INTO exam_session
    (profile_id, mode, variant_id, category_id, sign_id, locale, time_limit_sec, errors_allowed, total)
  VALUES (
    sqlc.arg(profile_id),
    sqlc.arg(mode),
    sqlc.arg(variant_id),
    sqlc.arg(category_id),
    sqlc.arg(sign_id),
    sqlc.arg(locale),
    sqlc.arg(time_limit_sec),
    sqlc.arg(errors_allowed),
    COALESCE(cardinality(sqlc.arg(question_ids)::uuid[]), 0)
  )
  RETURNING *
), assigned AS (
  INSERT INTO session_question (session_id, question_id, position)
  SELECT created.id, questions.question_id, questions.position::smallint
  FROM created
  CROSS JOIN unnest(sqlc.arg(question_ids)::uuid[])
    WITH ORDINALITY AS questions(question_id, position)
  RETURNING session_id
)
SELECT created.*
FROM created
CROSS JOIN (SELECT count(*) FROM assigned) persisted;

-- name: GetExamSession :one
SELECT * FROM exam_session WHERE id = $1;

-- name: FinishExamSession :one
UPDATE exam_session
SET finished_at = now(), status = $2, score = $3, stopped_reason = $4
WHERE id = $1
RETURNING *;

-- name: InsertSessionAnswer :one
INSERT INTO session_answer (session_id, question_id, answer_id, is_correct, position)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetSessionAnswer :one
SELECT * FROM session_answer
WHERE session_id = sqlc.arg(session_id) AND question_id = sqlc.arg(question_id);

-- name: GetSessionQuestion :one
SELECT * FROM session_question
WHERE session_id = sqlc.arg(session_id) AND question_id = sqlc.arg(question_id);

-- name: ListSessionAnswers :many
SELECT * FROM session_answer WHERE session_id = $1 ORDER BY position;

-- name: ListSessionQuestionsWithAnswers :many
SELECT
  sq.question_id,
  sq.position,
  sa.answer_id AS user_answer_id,
  sa.is_correct,
  q.correct_answer_id
FROM session_question sq
JOIN question q ON q.id = sq.question_id
LEFT JOIN session_answer sa
  ON sa.session_id = sq.session_id AND sa.question_id = sq.question_id
WHERE sq.session_id = $1
ORDER BY sq.position;

-- name: CountSessionAnswers :one
SELECT
  count(*)::int AS total_answered,
  count(*) FILTER (WHERE is_correct)::int AS correct_count
FROM session_answer WHERE session_id = $1;

-- name: ListMySessions :many
SELECT * FROM exam_session
WHERE profile_id = sqlc.arg(profile_id)
ORDER BY started_at DESC LIMIT sqlc.arg(limit_count);

-- name: GetLimitConfig :one
SELECT * FROM limit_config WHERE key = $1;

-- name: CountPracticeAnswersToday :one
SELECT count(*)::int FROM session_answer sa
JOIN exam_session es ON es.id = sa.session_id
WHERE es.profile_id = sqlc.arg(profile_id) AND es.mode = 'practice'
  AND sa.answered_at >= sqlc.arg(since);

-- name: UpsertVariantProgress :one
INSERT INTO variant_progress (profile_id, variant_id, best_correct, attempts, completed_at)
VALUES ($1, $2, $3, 1, $4)
ON CONFLICT (profile_id, variant_id) DO UPDATE SET
  best_correct = GREATEST(variant_progress.best_correct, EXCLUDED.best_correct),
  attempts = variant_progress.attempts + 1,
  completed_at = COALESCE(variant_progress.completed_at, EXCLUDED.completed_at)
RETURNING *;

-- name: ListVariantProgressForProfile :many
SELECT * FROM variant_progress WHERE profile_id = $1;

-- name: GetVariantByID :one
SELECT * FROM variant WHERE id = $1;
