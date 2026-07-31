DELETE FROM limit_config WHERE key IN ('tg_quiz_seconds', 'tg_quiz_questions');

DROP TABLE IF EXISTS telegram_quiz_poll;
DROP TABLE IF EXISTS telegram_quiz_participant;

ALTER TABLE telegram_quiz_session
  DROP COLUMN IF EXISTS mode,
  DROP COLUMN IF EXISTS question_no,
  DROP COLUMN IF EXISTS total_questions;
