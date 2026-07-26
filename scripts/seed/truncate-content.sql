-- Wipe CONTENT tables only (questions / variants / signs / images / explanations).
-- Keeps learners, payments, admin users, limit_config, feature flags, tariffs.
-- CMS site_settings is cleared so empty → FE i18n fallback (operator can re-save).
--
-- Usage (compose stack up):
--   make seed-reset-content
--   make seed-dev

TRUNCATE TABLE
  explanation_feedback,
  explanation_translation,
  explanation,
  question_sign,
  sign_translation,
  sign,
  sign_group_translation,
  sign_group,
  variant_question,
  variant,
  answer_translation,
  question_translation,
  answer,
  question,
  image,
  category_translation,
  category,
  site_settings
RESTART IDENTITY CASCADE;
