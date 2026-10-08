-- Bot password reset: proving the Telegram identity (linked account or a
-- matching contact) no longer verifies the reset on its own. A contact can
-- reach the bot chat for unrelated reasons (the Mini App's phone share), and
-- a victim can be lured into opening an attacker-started reset link. The bot
-- now asks «Ha, men» / «Yo'q»; the buttons carry a per-reset nonce whose hash
-- is stored here. NULL = no question outstanding. See
-- auth.AnswerTelegramPasswordResetConfirm.
ALTER TABLE password_reset_token ADD COLUMN confirm_nonce_hash text;

CREATE UNIQUE INDEX password_reset_token_confirm_nonce_idx
  ON password_reset_token (confirm_nonce_hash)
  WHERE confirm_nonce_hash IS NOT NULL;
