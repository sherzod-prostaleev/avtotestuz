-- name: SaveQuestion :exec
INSERT INTO saved_question (profile_id, question_id)
VALUES ($1, $2)
ON CONFLICT (profile_id, question_id) DO NOTHING;

-- name: UnsaveQuestion :exec
DELETE FROM saved_question WHERE profile_id = sqlc.arg(profile_id) AND question_id = sqlc.arg(question_id);

-- name: ListSavedQuestions :many
SELECT sq.question_id, sq.created_at
FROM saved_question sq
JOIN question q ON q.id = sq.question_id AND q.validation_status = 'valid'
WHERE sq.profile_id = $1
ORDER BY sq.created_at DESC;

-- name: GetStreak :one
SELECT * FROM streak WHERE profile_id = $1;

-- name: UpsertStreak :one
INSERT INTO streak (profile_id, current, best, last_active_date, daily_goal, today_done)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (profile_id) DO UPDATE SET
  current = EXCLUDED.current,
  best = EXCLUDED.best,
  last_active_date = EXCLUDED.last_active_date,
  daily_goal = EXCLUDED.daily_goal,
  today_done = EXCLUDED.today_done
RETURNING *;
