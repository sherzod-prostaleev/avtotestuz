DELETE FROM feature_flag WHERE key = 'telegram_daily_reminder';
INSERT INTO feature_flag (key, type, value_json, description) VALUES
  ('telegram_dm_digest', 'boolean', 'true'::jsonb,
   'Daily soft due/streak Telegram DM digests for linked accounts')
ON CONFLICT (key) DO NOTHING;
DROP TABLE IF EXISTS telegram_daily_question;
DROP TABLE IF EXISTS telegram_bot_user;
