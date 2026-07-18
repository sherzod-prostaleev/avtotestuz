-- name: GetExplanationTranslationForFeedback :one
-- Used only to validate that a question has an explanation before recording
-- feedback against it — returns the explanation_id regardless of locale/status,
-- since feedback is about the question's explanation as a whole, not one locale.
SELECT e.id AS explanation_id
FROM explanation e
WHERE e.question_id = sqlc.arg(question_id)
LIMIT 1;

-- name: UpsertExplanationFeedback :exec
INSERT INTO explanation_feedback (profile_id, explanation_id, helpful)
VALUES ($1, $2, $3)
ON CONFLICT (profile_id, explanation_id) DO UPDATE SET helpful = EXCLUDED.helpful;

-- name: GetQuestionForDraft :one
SELECT q.id, q.category_id, c.code AS category_code,
       COALESCE(qt.text, qft.text, '') AS question_text
FROM question q
JOIN category c ON c.id = q.category_id
LEFT JOIN question_translation qt
       ON qt.question_id = q.id AND qt.locale = 'uz-Latn' AND qt.status = 'verified'
LEFT JOIN question_translation qft
       ON qft.question_id = q.id AND qft.locale = 'uz-Latn'
WHERE q.id = sqlc.arg(id);

-- name: GetCorrectAnswerTextForDraft :one
SELECT COALESCE(at.text, aft.text, '') AS answer_text
FROM answer a
LEFT JOIN answer_translation at
       ON at.answer_id = a.id AND at.locale = 'uz-Latn' AND at.status = 'verified'
LEFT JOIN answer_translation aft
       ON aft.answer_id = a.id AND aft.locale = 'uz-Latn'
WHERE a.question_id = sqlc.arg(question_id) AND a.is_correct = true;

-- name: InsertDraftExplanation :one
INSERT INTO explanation (question_id, legal_refs)
VALUES ($1, '[]'::jsonb)
ON CONFLICT (question_id) DO UPDATE SET legal_refs = explanation.legal_refs
RETURNING id;

-- name: InsertDraftTranslation :exec
INSERT INTO explanation_translation (explanation_id, locale, blocks, status, source)
VALUES ($1, $2, $3, 'draft', 'ai-stub')
ON CONFLICT (explanation_id, locale) DO UPDATE
  SET blocks = EXCLUDED.blocks, status = 'draft', source = 'ai-stub';

-- name: GetExplanationTranslationByExplanationAndLocale :one
SELECT * FROM explanation_translation
WHERE explanation_id = sqlc.arg(explanation_id) AND locale = sqlc.arg(locale);

-- name: VerifyExplanationTranslation :exec
UPDATE explanation_translation
SET status = 'verified', verified_by = sqlc.arg(verified_by), verified_at = now()
WHERE explanation_id = sqlc.arg(explanation_id) AND locale = sqlc.arg(locale);

-- name: GetExplanationIDByQuestionID :one
SELECT id FROM explanation WHERE question_id = $1;
