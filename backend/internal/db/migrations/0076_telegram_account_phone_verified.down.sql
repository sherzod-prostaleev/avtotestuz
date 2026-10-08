ALTER TABLE password_reset_token DROP COLUMN IF EXISTS verified_tg_user_id;
ALTER TABLE telegram_account DROP COLUMN IF EXISTS phone_verified_at;
