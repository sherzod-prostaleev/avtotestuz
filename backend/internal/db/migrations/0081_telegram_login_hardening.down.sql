DELETE FROM feature_flag WHERE key = 'telegram_login';

DROP INDEX IF EXISTS telegram_login_request_profile_idx;

ALTER TABLE telegram_login_request
  DROP CONSTRAINT IF EXISTS telegram_login_request_contact_phone_chk,
  DROP COLUMN IF EXISTS contact_phone,
  DROP COLUMN IF EXISTS opened_tg_user_id;
