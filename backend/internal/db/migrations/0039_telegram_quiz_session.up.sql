-- M4-07: on-demand Telegram group/DM quiz sessions + chat registry.
-- See docs/superpowers/specs/2026-07-26-m4-07-telegram-quiz-growth.md.

CREATE TABLE telegram_chat (
  chat_id    bigint PRIMARY KEY,
  title      text NOT NULL DEFAULT '',
  chat_type  text NOT NULL DEFAULT '',
  bot_status text NOT NULL DEFAULT 'member',
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE telegram_quiz_session (
  id                    uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  chat_id               bigint NOT NULL,
  started_by_tg_user_id bigint NOT NULL,
  active                boolean NOT NULL DEFAULT true,
  question_id           uuid REFERENCES question(id) ON DELETE SET NULL,
  awaiting_answer       boolean NOT NULL DEFAULT false,
  answer_message_id     bigint NOT NULL DEFAULT 0,
  asked_count           int NOT NULL DEFAULT 0,
  correct_count         int NOT NULL DEFAULT 0,
  last_activity_at      timestamptz NOT NULL DEFAULT now(),
  created_at            timestamptz NOT NULL DEFAULT now()
);

-- One live session per chat; historical rows stay for stats.
CREATE UNIQUE INDEX telegram_quiz_session_one_active_idx
  ON telegram_quiz_session (chat_id)
  WHERE active;

CREATE INDEX telegram_quiz_session_chat_idx
  ON telegram_quiz_session (chat_id, created_at DESC);

INSERT INTO feature_flag (key, type, value_json, description) VALUES
  ('telegram_quiz', 'boolean', 'true'::jsonb,
   'On-demand Telegram /quiz sessions (groups + DM)'),
  ('telegram_dm_digest', 'boolean', 'true'::jsonb,
   'Daily soft due/streak Telegram DM digests for linked accounts')
ON CONFLICT (key) DO NOTHING;
