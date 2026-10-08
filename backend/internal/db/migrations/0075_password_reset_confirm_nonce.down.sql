DROP INDEX IF EXISTS password_reset_token_confirm_nonce_idx;
ALTER TABLE password_reset_token DROP COLUMN IF EXISTS confirm_nonce_hash;
