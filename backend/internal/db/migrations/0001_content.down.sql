DROP TABLE IF EXISTS explanation_translation, explanation, question_sign,
  sign_translation, sign, sign_group_translation, sign_group,
  variant_question, variant, answer_translation, question_translation CASCADE;
ALTER TABLE IF EXISTS question DROP CONSTRAINT IF EXISTS question_correct_answer_fk;
DROP TABLE IF EXISTS answer, question, image, category_translation, category CASCADE;
DROP DOMAIN IF EXISTS locale_code;
