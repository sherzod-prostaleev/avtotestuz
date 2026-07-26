-- name: GetQuestionMemory :one
SELECT * FROM question_memory
WHERE profile_id = sqlc.arg(profile_id) AND question_id = sqlc.arg(question_id);

-- name: UpsertQuestionMemory :one
INSERT INTO question_memory
  (profile_id, question_id, stability, difficulty, due_at, last_reviewed_at, reps, lapses, state)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (profile_id, question_id) DO UPDATE SET
  stability        = EXCLUDED.stability,
  difficulty       = EXCLUDED.difficulty,
  due_at           = EXCLUDED.due_at,
  last_reviewed_at = EXCLUDED.last_reviewed_at,
  reps             = EXCLUDED.reps,
  lapses           = EXCLUDED.lapses,
  state            = EXCLUDED.state
RETURNING *;

-- name: ListDueQuestions :many
SELECT qm.question_id, q.category_id
FROM question_memory qm
JOIN question q ON q.id = qm.question_id AND q.validation_status = 'valid'
WHERE qm.profile_id = sqlc.arg(profile_id) AND qm.due_at <= now()
ORDER BY qm.due_at ASC
LIMIT sqlc.arg(limit_count);

-- name: CountDueQuestions :one
SELECT count(*)::int FROM question_memory
WHERE profile_id = $1 AND due_at <= now();

-- name: PassRateByReadinessBucket :many
-- Empirical P(pass | readiness bucket) from finished exam-like sessions that
-- stored readiness_pct_at_finish. Buckets are 0,10,...,90.
SELECT
  (readiness_pct_at_finish / 10) * 10 AS bucket_lo,
  count(*)::int AS n,
  count(*) FILTER (WHERE status = 'passed')::int AS passed
FROM exam_session
WHERE readiness_pct_at_finish IS NOT NULL
  AND mode IN ('exam', 'grand_mock', 'placement')
  AND status IN ('passed', 'failed')
GROUP BY 1
ORDER BY 1;

-- name: CountStudiedQuestions :one
-- How many DISTINCT questions the profile has ever been graded on. Used as
-- the Grand Mock volume floor (session.MockEligibility): question_memory is
-- keyed (profile_id, question_id), so unlike category_mastery.seen — which
-- counts answer events — this cannot be inflated by re-answering the same
-- easy question over and over.
SELECT count(*)::int FROM question_memory WHERE profile_id = $1;

-- name: GetMistakeBankSummary :one
SELECT
  count(*) FILTER (WHERE qm.due_at <= now())::int AS due_count,
  count(*)::int AS total_bank_count,
  (min(qm.due_at) FILTER (WHERE qm.due_at > now()))::timestamptz AS next_due_at
FROM question_memory qm
JOIN question q ON q.id = qm.question_id AND q.validation_status = 'valid'
WHERE qm.profile_id = $1 AND qm.lapses > 0;

-- name: GetQuestionCategoryID :one
SELECT category_id FROM question WHERE id = $1;

-- name: UpsertCategoryMastery :one
INSERT INTO category_mastery (profile_id, category_id, mastery, seen, correct, updated_at)
VALUES ($1, $2, $3, $4, $5, now())
ON CONFLICT (profile_id, category_id) DO UPDATE SET
  mastery    = EXCLUDED.mastery,
  seen       = EXCLUDED.seen,
  correct    = EXCLUDED.correct,
  updated_at = now()
RETURNING *;

-- name: GetCategoryMastery :one
SELECT * FROM category_mastery
WHERE profile_id = sqlc.arg(profile_id) AND category_id = sqlc.arg(category_id);

-- name: ListCategoryMasteryForProfile :many
SELECT * FROM category_mastery WHERE profile_id = $1;

-- name: CountValidQuestionsByCategory :many
SELECT category_id, count(*)::int AS question_count
FROM question WHERE validation_status = 'valid'
GROUP BY category_id;

-- name: CountStudiedQuestionsByCategory :many
-- Distinct questions the profile has graded at least once, per category.
-- Used with category bank size so mastery/readiness cannot inflate by
-- re-drilling a tiny subset of easy questions.
SELECT q.category_id, count(*)::int AS studied_count
FROM question_memory qm
JOIN question q ON q.id = qm.question_id AND q.validation_status = 'valid'
WHERE qm.profile_id = $1
GROUP BY q.category_id;

-- name: CountValidQuestions :one
-- Bank size, used to turn the Grand Mock volume floor
-- (limit_config.grand_mock_min_studied_pct) into an absolute question count so
-- the requirement keeps its meaning as content grows.
SELECT count(*)::int FROM question WHERE validation_status = 'valid';
