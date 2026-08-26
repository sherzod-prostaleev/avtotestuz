-- name: ListVariantQuestionIDsOrdered :many
SELECT vq.question_id
FROM variant_question vq
JOIN question q ON q.id = vq.question_id AND q.validation_status = 'valid'
WHERE vq.variant_id = $1
ORDER BY vq.position;

-- name: RandomQuestionIDs :many
WITH pivot AS MATERIALIZED (
  SELECT gen_random_uuid() AS id
), forward AS (
  SELECT q.id
  FROM question q CROSS JOIN pivot p
  WHERE q.validation_status = 'valid' AND q.id >= p.id
  ORDER BY q.id
  LIMIT $1
), wrapped AS (
  SELECT q.id
  FROM question q CROSS JOIN pivot p
  WHERE q.validation_status = 'valid' AND q.id < p.id
  ORDER BY q.id
  LIMIT $1
)
SELECT id FROM forward
UNION ALL
SELECT id FROM wrapped
LIMIT $1;

-- name: RandomQuestionIDsByCategory :many
WITH pivot AS MATERIALIZED (
  SELECT gen_random_uuid() AS id
), forward AS (
  SELECT q.id
  FROM question q CROSS JOIN pivot p
  WHERE q.validation_status = 'valid'
    AND q.category_id = sqlc.arg(category_id)
    AND q.id >= p.id
  ORDER BY q.id
  LIMIT sqlc.arg(limit_count)
), wrapped AS (
  SELECT q.id
  FROM question q CROSS JOIN pivot p
  WHERE q.validation_status = 'valid'
    AND q.category_id = sqlc.arg(category_id)
    AND q.id < p.id
  ORDER BY q.id
  LIMIT sqlc.arg(limit_count)
)
SELECT id FROM forward
UNION ALL
SELECT id FROM wrapped
LIMIT sqlc.arg(limit_count);

-- name: RandomQuestionIDsBySign :many
WITH pivot AS MATERIALIZED (
  SELECT gen_random_uuid() AS id
), forward AS (
  SELECT q.id
  FROM question_sign qs
  JOIN question q ON q.id = qs.question_id
  CROSS JOIN pivot p
  WHERE q.validation_status = 'valid'
    AND qs.sign_id = sqlc.arg(sign_id)
    AND q.id >= p.id
  ORDER BY q.id
  LIMIT sqlc.arg(limit_count)
), wrapped AS (
  SELECT q.id
  FROM question_sign qs
  JOIN question q ON q.id = qs.question_id
  CROSS JOIN pivot p
  WHERE q.validation_status = 'valid'
    AND qs.sign_id = sqlc.arg(sign_id)
    AND q.id < p.id
  ORDER BY q.id
  LIMIT sqlc.arg(limit_count)
)
SELECT id FROM forward
UNION ALL
SELECT id FROM wrapped
LIMIT sqlc.arg(limit_count);

-- name: RandomQuestionIDsByVariantRange :many
-- Draws across a contiguous span of bilets so a learner can mix-review the
-- range they have already worked through, which one-bilet-at-a-time cannot do.
WITH pivot AS MATERIALIZED (
  SELECT gen_random_uuid() AS id
), forward AS (
  SELECT q.id
  FROM question q CROSS JOIN pivot p
  WHERE q.validation_status = 'valid'
    AND q.id >= p.id
    AND EXISTS (
      SELECT 1
      FROM variant_question vq
      JOIN variant v ON v.id = vq.variant_id
      WHERE vq.question_id = q.id
        AND v.number BETWEEN sqlc.arg(from_number) AND sqlc.arg(to_number)
    )
  ORDER BY q.id
  LIMIT sqlc.arg(limit_count)
), wrapped AS (
  SELECT q.id
  FROM question q CROSS JOIN pivot p
  WHERE q.validation_status = 'valid'
    AND q.id < p.id
    AND EXISTS (
      SELECT 1
      FROM variant_question vq
      JOIN variant v ON v.id = vq.variant_id
      WHERE vq.question_id = q.id
        AND v.number BETWEEN sqlc.arg(from_number) AND sqlc.arg(to_number)
    )
  ORDER BY q.id
  LIMIT sqlc.arg(limit_count)
)
SELECT id FROM forward
UNION ALL
SELECT id FROM wrapped
LIMIT sqlc.arg(limit_count);

-- name: RandomQuestionIDsByImagePresence :many
-- has_image=true selects illustrated questions, false selects text-only ones.
WITH pivot AS MATERIALIZED (
  SELECT gen_random_uuid() AS id
), forward AS (
  SELECT q.id
  FROM question q CROSS JOIN pivot p
  WHERE q.validation_status = 'valid'
    AND (q.image_id IS NOT NULL) = sqlc.arg(has_image)::boolean
    AND q.id >= p.id
  ORDER BY q.id
  LIMIT sqlc.arg(limit_count)
), wrapped AS (
  SELECT q.id
  FROM question q CROSS JOIN pivot p
  WHERE q.validation_status = 'valid'
    AND (q.image_id IS NOT NULL) = sqlc.arg(has_image)::boolean
    AND q.id < p.id
  ORDER BY q.id
  LIMIT sqlc.arg(limit_count)
)
SELECT id FROM forward
UNION ALL
SELECT id FROM wrapped
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
    (profile_id, mode, variant_id, category_id, sign_id, locale, time_limit_sec, errors_allowed, total, ordered_from)
  VALUES (
    sqlc.arg(profile_id),
    sqlc.arg(mode),
    sqlc.arg(variant_id),
    sqlc.arg(category_id),
    sqlc.arg(sign_id),
    sqlc.arg(locale),
    sqlc.arg(time_limit_sec),
    sqlc.arg(errors_allowed),
    COALESCE(cardinality(sqlc.arg(question_ids)::uuid[]), 0),
    sqlc.narg(ordered_from)
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
SET finished_at = now(),
    status = $2,
    score = $3,
    stopped_reason = $4,
    readiness_pct_at_finish = COALESCE(sqlc.narg(readiness_pct_at_finish), readiness_pct_at_finish)
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

-- name: OrderedQuestionIDsByCategory :many
-- The topic in its source order, so "all questions of one topic" is the same
-- walk every time and can be resumed from an offset.
--
-- source_ext_id is 'avtoimtihon-<N>' for all 1260 questions in the bank and
-- every N is distinct, which makes the numeric suffix a total, unique ordering
-- -- and the same numbering the source material itself uses, so it means
-- something to the teacher reading it. NULLIF guards an id with no digits: it
-- sorts last instead of failing the cast, and source_ext_id then id keep the
-- order total even then. No index: the whole bank is 1260 rows.
SELECT q.id
FROM question q
WHERE q.validation_status = 'valid'
  AND q.category_id = sqlc.arg(category_id)
ORDER BY NULLIF(regexp_replace(q.source_ext_id, '\D', '', 'g'), '')::bigint NULLS LAST,
         q.source_ext_id,
         q.id
OFFSET sqlc.arg(skip)
LIMIT sqlc.arg(limit_count);

-- name: CountValidQuestionsInCategory :one
-- Named "InCategory", not "ByCategory": learning.sql already owns
-- CountValidQuestionsByCategory, which counts every category at once for the
-- practice screen. This one answers about a single topic, which is what the
-- ordered draw needs to know before it can decide whether the cursor has run
-- off the end and should wrap.
SELECT count(*)::int FROM question
WHERE category_id = sqlc.arg(category_id) AND validation_status = 'valid';

-- name: GetPracticeCursor :one
SELECT next_index FROM practice_cursor
WHERE profile_id = sqlc.arg(profile_id) AND category_id = sqlc.arg(category_id);

-- name: AdvancePracticeCursor :exec
-- Moves a profile's place in a topic to the end of what it has actually worked
-- through, which is not the same as the furthest question it has touched.
--
-- `prefix` is the contiguous run of positions answered from the start of this
-- session. position - row_number() is 0 for exactly that run: the first gap
-- makes it positive and it never returns to 0. This is what stops a student who
-- scrolls the question navigator to the end and answers the last chip from
-- marking a 337-question topic complete -- which would wrap the walk and
-- discard the class's real position -- and stops any forward jump from silently
-- burying the questions it skipped over.
--
-- The ordered_from <= current guard drops answers that belong to a walk the
-- class has already left behind. Practice sessions are left open routinely and
-- the history makes them reopenable, so without it one answer in a month-old
-- session would write that old walk's position over today's and skip
-- everything in between. A session of the current walk always satisfies it:
-- ordered_from is the cursor as it stood when the session was drawn, and the
-- cursor only ever moves forward from there.
--
-- GREATEST keeps the write monotone, so answering out of order within the
-- current session, or two answers racing, can never rewind the class.
WITH prefix AS (
  SELECT COALESCE(MAX(position), 0)::int AS done
  FROM (
    SELECT position, position - (ROW_NUMBER() OVER (ORDER BY position))::int AS gap
    FROM session_answer WHERE session_id = sqlc.arg(session_id)
  ) runs
  WHERE gap = 0
), current AS (
  SELECT COALESCE((SELECT next_index FROM practice_cursor
                    WHERE profile_id = sqlc.arg(profile_id)
                      AND category_id = sqlc.arg(category_id)), 0)::int AS at
)
INSERT INTO practice_cursor AS pc (profile_id, category_id, next_index)
SELECT sqlc.arg(profile_id), sqlc.arg(category_id), sqlc.arg(ordered_from)::int + prefix.done
FROM prefix, current
WHERE sqlc.arg(ordered_from)::int <= current.at
ON CONFLICT (profile_id, category_id) DO UPDATE
SET next_index = GREATEST(pc.next_index, EXCLUDED.next_index),
    updated_at = now();

-- name: ResetPracticeCursor :exec
-- Deleting rather than zeroing: no row IS the start, so this keeps the table
-- free of rows that say nothing and makes ListPracticeProgress omit them
-- without a filter.
DELETE FROM practice_cursor
WHERE profile_id = sqlc.arg(profile_id) AND category_id = sqlc.arg(category_id);

-- name: ListPracticeProgress :many
-- Returns the category's code as well as its id, because the code is the only
-- identifier the practice screen holds: GET /categories answers with code,
-- name, sort_order and question_count and no uuid at all (content.CategoryDTO).
-- A progress list keyed only by uuid would be unmatchable by the one screen
-- that needs it.
SELECT pc.category_id,
       c.code AS category_code,
       pc.next_index,
       (SELECT count(*) FROM question q
         WHERE q.category_id = pc.category_id AND q.validation_status = 'valid')::int AS total
FROM practice_cursor pc
JOIN category c ON c.id = pc.category_id
WHERE pc.profile_id = sqlc.arg(profile_id)
ORDER BY c.sort_order, c.code;
