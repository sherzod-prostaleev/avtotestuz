-- name: ListCategories :many
-- question_count lets the practice picker state how much material a topic
-- actually holds instead of guessing; categories range from tens to hundreds.
SELECT c.id, c.code, c.sort_order,
       COALESCE(t.name, ft.name, '') AS name,
       (t.name IS NULL)::bool AS fallback_used,
       (SELECT count(*) FROM question q
         WHERE q.category_id = c.id AND q.validation_status = 'valid')::int AS question_count
FROM category c
LEFT JOIN category_translation t
       ON t.category_id = c.id AND t.locale = $1 AND t.status = 'verified'
LEFT JOIN category_translation ft
       ON ft.category_id = c.id AND ft.locale = 'uz-Latn' AND ft.status = 'verified'
ORDER BY c.sort_order, c.code;

-- name: ListVariants :many
SELECT v.id, v.number, count(vq.question_id)::int AS question_count
FROM variant v
LEFT JOIN variant_question vq ON vq.variant_id = v.id
GROUP BY v.id, v.number
ORDER BY v.number;

-- name: GetVariantByNumber :one
SELECT * FROM variant WHERE number = $1;

-- name: ListVariantQuestions :many
SELECT q.id, vq.position, c.code AS category_code,
       img.storage_key AS image_key,
       COALESCE(qt.text, qft.text, '') AS text,
       (qt.text IS NULL)::bool AS fallback_used
FROM variant_question vq
JOIN question q ON q.id = vq.question_id AND q.validation_status = 'valid'
JOIN category c ON c.id = q.category_id
LEFT JOIN image img ON img.id = q.image_id
LEFT JOIN question_translation qt
       ON qt.question_id = q.id AND qt.locale = sqlc.arg(locale) AND qt.status = 'verified'
LEFT JOIN question_translation qft
       ON qft.question_id = q.id AND qft.locale = 'uz-Latn' AND qft.status = 'verified'
WHERE vq.variant_id = sqlc.arg(variant_id)
ORDER BY vq.position;

-- name: ListAnswersByQuestionIDs :many
SELECT a.id, a.question_id, a.position,
       aimg.storage_key AS image_key,
       COALESCE(at.text, aft.text, '') AS text,
       -- Answer-shuffling reads this, never the localized text: whether a
       -- question's options may be reordered is a property of the question,
       -- and deciding it from whichever locale the reader happens to use made
       -- the same question shuffle in Russian and stay put in Uzbek.
       COALESCE(aft.text, at.text, '') AS text_uz_latn,
       (at.text IS NULL)::bool AS fallback_used
FROM answer a
LEFT JOIN image aimg ON aimg.id = a.image_id
LEFT JOIN answer_translation at
       ON at.answer_id = a.id AND at.locale = sqlc.arg(locale) AND at.status = 'verified'
LEFT JOIN answer_translation aft
       ON aft.answer_id = a.id AND aft.locale = 'uz-Latn' AND aft.status = 'verified'
WHERE a.question_id = ANY(sqlc.arg(question_ids)::uuid[])
ORDER BY a.question_id, a.position;

-- name: GetQuestion :one
SELECT q.id, c.code AS category_code,
       img.storage_key AS image_key,
       COALESCE(qt.text, qft.text, '') AS text,
       (qt.text IS NULL)::bool AS fallback_used
FROM question q
JOIN category c ON c.id = q.category_id
LEFT JOIN image img ON img.id = q.image_id
LEFT JOIN question_translation qt
       ON qt.question_id = q.id AND qt.locale = sqlc.arg(locale) AND qt.status = 'verified'
LEFT JOIN question_translation qft
       ON qft.question_id = q.id AND qft.locale = 'uz-Latn' AND qft.status = 'verified'
WHERE q.id = sqlc.arg(id) AND q.validation_status = 'valid';

-- name: ListQuestionsByIDs :many
SELECT q.id, c.code AS category_code,
       img.storage_key AS image_key,
       COALESCE(qt.text, qft.text, '') AS text,
       (qt.text IS NULL)::bool AS fallback_used
FROM question q
JOIN category c ON c.id = q.category_id
LEFT JOIN image img ON img.id = q.image_id
LEFT JOIN question_translation qt
       ON qt.question_id = q.id AND qt.locale = sqlc.arg(locale) AND qt.status = 'verified'
LEFT JOIN question_translation qft
       ON qft.question_id = q.id AND qft.locale = 'uz-Latn' AND qft.status = 'verified'
WHERE q.id = ANY(sqlc.arg(ids)::uuid[]) AND q.validation_status = 'valid';

-- name: ListQuestionSigns :many
SELECT s.code, simg.storage_key AS image_key,
       COALESCE(st.name, sft.name, '') AS name
FROM question_sign qs
JOIN sign s ON s.id = qs.sign_id
LEFT JOIN image simg ON simg.id = s.image_id
LEFT JOIN sign_translation st
       ON st.sign_id = s.id AND st.locale = sqlc.arg(locale) AND st.status = 'verified'
LEFT JOIN sign_translation sft
       ON sft.sign_id = s.id AND sft.locale = 'uz-Latn' AND sft.status = 'verified'
WHERE qs.question_id = sqlc.arg(question_id)
ORDER BY s.code;

-- name: ListQuestionSignsByQuestionIDs :many
SELECT qs.question_id, s.code, simg.storage_key AS image_key,
       COALESCE(st.name, sft.name, '') AS name
FROM question_sign qs
JOIN sign s ON s.id = qs.sign_id
LEFT JOIN image simg ON simg.id = s.image_id
LEFT JOIN sign_translation st
       ON st.sign_id = s.id AND st.locale = sqlc.arg(locale) AND st.status = 'verified'
LEFT JOIN sign_translation sft
       ON sft.sign_id = s.id AND sft.locale = 'uz-Latn' AND sft.status = 'verified'
WHERE qs.question_id = ANY(sqlc.arg(question_ids)::uuid[])
ORDER BY qs.question_id, s.code;

-- name: ListSignGroups :many
SELECT g.id, g.code, g.sort_order,
       COALESCE(t.name, ft.name, '') AS name
FROM sign_group g
LEFT JOIN sign_group_translation t
       ON t.sign_group_id = g.id AND t.locale = $1 AND t.status = 'verified'
LEFT JOIN sign_group_translation ft
       ON ft.sign_group_id = g.id AND ft.locale = 'uz-Latn' AND ft.status = 'verified'
ORDER BY g.sort_order, g.code;

-- name: ListSigns :many
SELECT s.id, s.code, g.code AS group_code,
       simg.storage_key AS image_key,
       COALESCE(st.name, sft.name, '') AS name,
       (SELECT count(*)::int FROM question_sign qs
         JOIN question q ON q.id = qs.question_id AND q.validation_status = 'valid'
        WHERE qs.sign_id = s.id) AS question_count
FROM sign s
JOIN sign_group g ON g.id = s.group_id
LEFT JOIN image simg ON simg.id = s.image_id
LEFT JOIN sign_translation st
       ON st.sign_id = s.id AND st.locale = sqlc.arg(locale) AND st.status = 'verified'
LEFT JOIN sign_translation sft
       ON sft.sign_id = s.id AND sft.locale = 'uz-Latn' AND sft.status = 'verified'
WHERE (sqlc.arg(group_code)::text = '' OR g.code = sqlc.arg(group_code)::text)
  AND (sqlc.arg(search)::text = ''
       OR s.code ILIKE '%' || sqlc.arg(search)::text || '%'
       OR COALESCE(st.name, sft.name, '') ILIKE '%' || sqlc.arg(search)::text || '%')
ORDER BY g.sort_order, s.sort_order, s.code;

-- name: GetSignByCode :one
SELECT s.id, s.code, g.code AS group_code,
       simg.storage_key AS image_key,
       COALESCE(st.name, sft.name, '') AS name,
       COALESCE(st.description, sft.description, '') AS description
FROM sign s
JOIN sign_group g ON g.id = s.group_id
LEFT JOIN image simg ON simg.id = s.image_id
LEFT JOIN sign_translation st
       ON st.sign_id = s.id AND st.locale = sqlc.arg(locale) AND st.status = 'verified'
LEFT JOIN sign_translation sft
       ON sft.sign_id = s.id AND sft.locale = 'uz-Latn' AND sft.status = 'verified'
WHERE s.code = sqlc.arg(code);

-- name: ListSignQuestionIDs :many
SELECT qs.question_id
FROM question_sign qs
JOIN question q ON q.id = qs.question_id AND q.validation_status = 'valid'
WHERE qs.sign_id = $1;

-- name: GetVerifiedExplanation :one
SELECT e.id, e.legal_refs,
       COALESCE(et.blocks, eft.blocks) AS blocks
FROM explanation e
LEFT JOIN explanation_translation et
       ON et.explanation_id = e.id AND et.locale = sqlc.arg(locale) AND et.status = 'verified'
LEFT JOIN explanation_translation eft
       ON eft.explanation_id = e.id AND eft.locale = 'uz-Latn' AND eft.status = 'verified'
WHERE e.question_id = sqlc.arg(question_id)
  AND COALESCE(et.blocks, eft.blocks) IS NOT NULL;

-- name: ListVerifiedExplanationsByQuestionIDs :many
SELECT e.question_id, e.legal_refs,
       COALESCE(et.blocks, eft.blocks) AS blocks
FROM explanation e
LEFT JOIN explanation_translation et
       ON et.explanation_id = e.id AND et.locale = sqlc.arg(locale) AND et.status = 'verified'
LEFT JOIN explanation_translation eft
       ON eft.explanation_id = e.id AND eft.locale = 'uz-Latn' AND eft.status = 'verified'
WHERE e.question_id = ANY(sqlc.arg(question_ids)::uuid[])
  AND COALESCE(et.blocks, eft.blocks) IS NOT NULL;
