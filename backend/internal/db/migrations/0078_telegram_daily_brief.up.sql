-- Daily «Kun savoli» reminder (.superpowers/sdd/daily-brief.md).
--
-- telegram_bot_user is the audience: every Telegram user who has a private
-- chat with the bot, linked or not. telegram_account only knows the minority
-- who linked a profile, so it cannot be the audience. No phone numbers and no
-- message contents are stored here.
CREATE TABLE telegram_bot_user (
  tg_user_id        bigint PRIMARY KEY,
  first_name        text NOT NULL DEFAULT '',
  username          text NOT NULL DEFAULT '',
  language_code     text NOT NULL DEFAULT '',
  first_seen_at     timestamptz NOT NULL DEFAULT now(),
  last_seen_at      timestamptz NOT NULL DEFAULT now(),
  reminders_enabled boolean NOT NULL DEFAULT true,
  -- Set when the user blocks the bot (my_chat_member kicked, or a 403 on
  -- send); cleared by any later private update from them.
  blocked_at        timestamptz,
  -- Tashkent calendar day of the last reminder this user was claimed for.
  -- Claiming and setting it is one UPDATE, which is what makes a crashed or
  -- concurrent run unable to send twice.
  last_reminder_on  date
);

-- One question per Tashkent day, shared by every recipient. The row is the
-- day's pick (replicas and restarts agree on it) and the history that keeps
-- a question from repeating until the eligible pool is used up.
CREATE TABLE telegram_daily_question (
  day         date PRIMARY KEY,
  question_id uuid NOT NULL REFERENCES question(id) ON DELETE CASCADE,
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX telegram_daily_question_question_idx ON telegram_daily_question (question_id, day DESC);

-- Seed the audience with the private chats we already know about: linked
-- accounts and solo /quiz chats (a positive chat id is a user's private
-- chat). Anyone who never actually opened the bot answers 403 / chat not
-- found on the first send and is marked blocked.
INSERT INTO telegram_bot_user (tg_user_id, language_code)
SELECT ta.tg_user_id, CASE WHEN p.locale_pref = 'ru' THEN 'ru' ELSE '' END
FROM telegram_account ta
LEFT JOIN profile p ON p.id = ta.profile_id
ON CONFLICT (tg_user_id) DO NOTHING;

INSERT INTO telegram_bot_user (tg_user_id)
SELECT DISTINCT chat_id FROM telegram_quiz_session WHERE chat_id > 0
ON CONFLICT (tg_user_id) DO NOTHING;

-- Starts OFF: prod sends nothing until the owner flips it in admin → flags.
INSERT INTO feature_flag (key, type, value_json, description) VALUES
  ('telegram_daily_reminder', 'boolean', 'false'::jsonb,
   'Daily 19:00 Tashkent «Kun savoli» quiz + personal line to every bot user (DM)')
ON CONFLICT (key) DO NOTHING;

-- The legacy linked-only due digest (cmd/tgdigest -send) is superseded by the
-- daily reminder; its flag would only suggest a second sender exists.
DELETE FROM feature_flag WHERE key = 'telegram_dm_digest';
