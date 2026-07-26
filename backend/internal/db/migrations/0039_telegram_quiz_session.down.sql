DELETE FROM feature_flag WHERE key IN ('telegram_quiz', 'telegram_dm_digest');
DROP TABLE IF EXISTS telegram_quiz_session;
DROP TABLE IF EXISTS telegram_chat;
