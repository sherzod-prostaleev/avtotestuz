DROP INDEX IF EXISTS telegram_bot_user_reminder_claim_idx;
ALTER TABLE telegram_bot_user DROP COLUMN IF EXISTS last_signup_pitch_on;
