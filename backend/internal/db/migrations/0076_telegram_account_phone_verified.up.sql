-- A Telegram link only proves "this Telegram account" when Telegram itself
-- vouched for the profile's phone (a signed requestContact share in the Mini
-- App, or the bot reset's contact + «Ha, men»). The legacy deep-link token
-- (/start <token>) links with no phone proof at all, so a token minted on one
-- profile can bind someone else's Telegram to it. phone_verified_at marks the
-- links that may sign in to the Mini App without a password; NULL = legacy or
-- unproven, still fine for bot digests. Existing rows start NULL on purpose:
-- one signed phone share re-proves them. See auth.TelegramWebAppLogin.
ALTER TABLE telegram_account ADD COLUMN phone_verified_at timestamptz;

-- Who confirmed a reset in the bot. pending_tg_user_id is cleared on verify
-- (its unique index must free the Telegram user for a new reset), so the
-- confirming user is kept here for CompletePasswordReset: only that user's
-- phone-verified link survives the reset.
ALTER TABLE password_reset_token ADD COLUMN verified_tg_user_id bigint;
