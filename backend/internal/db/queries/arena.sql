-- name: InsertArenaMatch :one
INSERT INTO arena_match (question_ids, question_time_sec, status, started_at)
VALUES ($1, $2, 'in_progress', now())
RETURNING *;

-- name: FinishArenaMatch :exec
UPDATE arena_match
SET status = 'finished', finished_at = now(), end_reason = $2
WHERE id = $1 AND status <> 'finished';

-- name: InsertArenaMatchPlayer :exec
INSERT INTO arena_match_player (
  match_id, profile_id, slot, locale, score, correct_count, total_response_ms,
  outcome, rating_before, rating_after, rating_delta
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11);

-- name: InsertArenaAnswer :exec
INSERT INTO arena_answer (
  match_id, profile_id, question_id, position, answer_id, is_correct, response_ms, points, answered_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);

-- name: ListArenaMatchesForProfile :many
SELECT m.id, m.status, m.finished_at, m.end_reason, m.created_at,
       p.slot, p.score, p.correct_count, p.outcome,
       p.rating_before, p.rating_after, p.rating_delta
FROM arena_match_player p
JOIN arena_match m ON m.id = p.match_id
WHERE p.profile_id = $1
ORDER BY p.joined_at DESC
LIMIT $2;

-- name: GetArenaMatchPlayer :one
SELECT * FROM arena_match_player
WHERE match_id = $1 AND profile_id = $2;

-- name: ListCorrectAnswerIDsForQuestions :many
SELECT question_id, id AS answer_id
FROM answer
WHERE question_id = ANY($1::uuid[]) AND is_correct = true;
