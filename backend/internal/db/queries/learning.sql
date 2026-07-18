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
