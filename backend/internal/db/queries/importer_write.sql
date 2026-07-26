-- name: UpsertImage :one
INSERT INTO image (storage_key, sha256, mime, source)
VALUES ($1, $2, $3, $4)
ON CONFLICT (sha256) DO UPDATE SET mime = EXCLUDED.mime
RETURNING id;

-- name: UpsertCategory :one
INSERT INTO category (code, sort_order)
VALUES ($1, $2)
ON CONFLICT (code) DO UPDATE SET sort_order = EXCLUDED.sort_order
RETURNING id;

-- name: UpsertCategoryTranslation :exec
INSERT INTO category_translation (category_id, locale, name, status)
VALUES ($1, $2, $3, $4)
ON CONFLICT (category_id, locale) DO UPDATE SET name = EXCLUDED.name, status = EXCLUDED.status;

-- name: UpsertSignGroup :one
INSERT INTO sign_group (code, sort_order)
VALUES ($1, $2)
ON CONFLICT (code) DO UPDATE SET sort_order = EXCLUDED.sort_order
RETURNING id;

-- name: UpsertSignGroupTranslation :exec
INSERT INTO sign_group_translation (sign_group_id, locale, name, status)
VALUES ($1, $2, $3, $4)
ON CONFLICT (sign_group_id, locale) DO UPDATE SET name = EXCLUDED.name, status = EXCLUDED.status;

-- name: UpsertSign :one
INSERT INTO sign (group_id, code, image_id, sort_order)
VALUES ($1, $2, $3, $4)
ON CONFLICT (code) DO UPDATE
  SET group_id = EXCLUDED.group_id, image_id = EXCLUDED.image_id, sort_order = EXCLUDED.sort_order
RETURNING id;

-- name: UpsertSignTranslation :exec
INSERT INTO sign_translation (sign_id, locale, name, description, status)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (sign_id, locale) DO UPDATE
  SET name = EXCLUDED.name, description = EXCLUDED.description, status = EXCLUDED.status;

-- name: UpsertQuestion :one
INSERT INTO question (source_ext_id, category_id, image_id, content_hash, source, validation_status, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, now())
ON CONFLICT (source_ext_id) DO UPDATE
  SET category_id = EXCLUDED.category_id,
      image_id = EXCLUDED.image_id,
      content_hash = EXCLUDED.content_hash,
      source = EXCLUDED.source,
      validation_status = EXCLUDED.validation_status,
      correct_answer_id = NULL,
      updated_at = now()
RETURNING id;

-- name: DeleteAnswersForQuestion :exec
DELETE FROM answer WHERE question_id = $1;

-- name: InsertAnswer :one
INSERT INTO answer (question_id, position, is_correct, image_id)
VALUES ($1, $2, $3, $4)
RETURNING id;

-- name: SetCorrectAnswer :exec
UPDATE question SET correct_answer_id = $2 WHERE id = $1;

-- name: UpsertQuestionTranslation :exec
INSERT INTO question_translation (question_id, locale, text, status, source)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (question_id, locale) DO UPDATE
  SET text = EXCLUDED.text, status = EXCLUDED.status, source = EXCLUDED.source;

-- name: InsertAnswerTranslation :exec
INSERT INTO answer_translation (answer_id, locale, text, status)
VALUES ($1, $2, $3, $4);

-- name: DeleteQuestionSigns :exec
DELETE FROM question_sign WHERE question_id = $1;

-- name: InsertQuestionSign :exec
INSERT INTO question_sign (question_id, sign_id) VALUES ($1, $2);

-- name: UpsertVariant :one
INSERT INTO variant (number, sort_order)
VALUES ($1, $2)
ON CONFLICT (number) DO UPDATE SET sort_order = EXCLUDED.sort_order
RETURNING id;

-- name: DeleteVariantQuestions :exec
DELETE FROM variant_question WHERE variant_id = $1;

-- name: InsertVariantQuestion :exec
INSERT INTO variant_question (variant_id, question_id, position) VALUES ($1, $2, $3);

-- name: GetQuestionIDBySourceExtID :one
SELECT id FROM question WHERE source_ext_id = $1;

-- name: UpsertExplanation :one
INSERT INTO explanation (question_id, legal_refs)
VALUES ($1, $2)
ON CONFLICT (question_id) DO UPDATE SET legal_refs = EXCLUDED.legal_refs
RETURNING id;

-- name: UpsertExplanationTranslation :exec
INSERT INTO explanation_translation (explanation_id, locale, blocks, status, source)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (explanation_id, locale) DO UPDATE
  SET blocks = EXCLUDED.blocks, status = EXCLUDED.status, source = EXCLUDED.source;
