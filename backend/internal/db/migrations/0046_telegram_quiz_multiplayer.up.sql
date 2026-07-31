-- Ko'p kishilik Telegram quizi: har bir ishtirokchi alohida hisoblanadi.
-- Spec: docs/superpowers/specs/2026-07-29-telegram-group-quiz-design.md

ALTER TABLE telegram_quiz_session
  ADD COLUMN total_questions int  NOT NULL DEFAULT 10,
  ADD COLUMN question_no     int  NOT NULL DEFAULT 0,
  ADD COLUMN mode            text NOT NULL DEFAULT 'solo';

CREATE TABLE telegram_quiz_participant (
  session_id     uuid   NOT NULL REFERENCES telegram_quiz_session(id) ON DELETE CASCADE,
  tg_user_id     bigint NOT NULL,
  display_name   text   NOT NULL DEFAULT '',
  answered_count int    NOT NULL DEFAULT 0,
  correct_count  int    NOT NULL DEFAULT 0,
  total_ms       bigint NOT NULL DEFAULT 0,
  first_seen_at  timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (session_id, tg_user_id)
);

CREATE INDEX telegram_quiz_participant_rank_idx
  ON telegram_quiz_participant (session_id, correct_count DESC, total_ms ASC);

-- poll_answer update'i faqat poll_id beradi — savolga qaytib bog'lash uchun.
CREATE TABLE telegram_quiz_poll (
  poll_id     text PRIMARY KEY,
  session_id  uuid NOT NULL REFERENCES telegram_quiz_session(id) ON DELETE CASCADE,
  question_id uuid REFERENCES question(id) ON DELETE SET NULL,
  question_no int  NOT NULL,
  correct_idx int  NOT NULL,
  opened_at   timestamptz NOT NULL DEFAULT now(),
  closed      boolean NOT NULL DEFAULT false
);

CREATE INDEX telegram_quiz_poll_session_idx
  ON telegram_quiz_poll (session_id, question_no);

-- Sozlamalar: /settings/limits admin sahifasidan deploy'siz o'zgartiriladi.
-- limit_config free/vip juftligi bu kalitlar uchun ma'nosiz — ikkalasi ham
-- bir xil qiymatni saqlaydi, o'qiyotgan kod faqat free_value ni oladi.
INSERT INTO limit_config (key, free_value, vip_value) VALUES
  ('tg_quiz_seconds',   10, 10),
  ('tg_quiz_questions', 10, 10)
ON CONFLICT (key) DO NOTHING;
