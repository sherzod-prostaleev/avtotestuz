ALTER TABLE telegram_bot_user
  DROP CONSTRAINT IF EXISTS telegram_bot_user_pending_referral_code_chk,
  DROP COLUMN IF EXISTS pending_referral_at,
  DROP COLUMN IF EXISTS pending_referral_code;

DROP TABLE IF EXISTS telegram_login_request;
