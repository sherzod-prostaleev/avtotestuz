-- name: CreateTelegramLoginRequest :one
INSERT INTO telegram_login_request (token_hash, browser_secret_hash, device, expires_at)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: DeleteStaleTelegramLoginRequests :exec
-- Retention: a request is useless a day past its expiry. Bounded per call so
-- a backlog never turns one website click into a long DELETE.
DELETE FROM telegram_login_request
WHERE id IN (
  SELECT id FROM telegram_login_request
  WHERE expires_at < now() - interval '1 day'
  LIMIT 500
);

-- name: GetTelegramLoginRequestByTokenHash :one
SELECT * FROM telegram_login_request WHERE token_hash = $1;

-- name: GetTelegramLoginRequestByTokenHashForUpdate :one
SELECT * FROM telegram_login_request WHERE token_hash = $1 FOR UPDATE;

-- name: ClearTelegramLoginPendingForTg :exec
-- A newer /start login_ of the same Telegram user owns their next contact
-- share; older requests drop their claim (and any open «✅ Kirish» question).
UPDATE telegram_login_request
SET pending_tg_user_id = NULL, confirm_nonce_hash = NULL
WHERE pending_tg_user_id = $1 AND id <> $2;

-- name: ClearAllTelegramLoginPendingForTg :exec
-- The Mini App phone share also posts a contact into the bot chat; once that
-- share has been used for the Mini App, it must not double as consent to a
-- website login the same user opened in the bot earlier.
UPDATE telegram_login_request
SET pending_tg_user_id = NULL, confirm_nonce_hash = NULL
WHERE pending_tg_user_id = $1 AND status = 'pending';

-- name: ArmTelegramLoginForTg :exec
UPDATE telegram_login_request
SET pending_tg_user_id = $2, confirm_nonce_hash = $3
WHERE id = $1;

-- name: GetLiveTelegramLoginByPendingTgForUpdate :one
SELECT * FROM telegram_login_request
WHERE pending_tg_user_id = $1 AND status = 'pending' AND expires_at > now()
FOR UPDATE;

-- name: GetTelegramLoginByConfirmNonceForUpdate :one
SELECT * FROM telegram_login_request WHERE confirm_nonce_hash = $1 FOR UPDATE;

-- name: ApproveTelegramLoginRequest :execrows
UPDATE telegram_login_request
SET status = 'approved', profile_id = $2, approved_tg_user_id = $3, approved_at = now(),
    pending_tg_user_id = NULL, confirm_nonce_hash = NULL
WHERE id = $1 AND status = 'pending';

-- name: EndTelegramLoginRequest :execrows
-- 'cancelled' (the learner said no) or 'blocked' (the account is banned).
UPDATE telegram_login_request
SET status = $2, pending_tg_user_id = NULL, confirm_nonce_hash = NULL
WHERE id = $1 AND status = 'pending';

-- name: ConsumeTelegramLoginRequest :execrows
UPDATE telegram_login_request
SET status = 'consumed', consumed_at = now()
WHERE id = $1 AND status = 'approved';

-- name: GetUserProfileByPhone :one
-- Telegram login / Mini App phone sign-in only ever match learners: a B2B
-- station's shadow profile (kind = 'station', phone 'st:<uuid>') must be
-- impossible to sign in to this way, whatever its phone column holds.
SELECT * FROM profile WHERE phone = $1 AND kind = 'user';

-- name: SetProfilePasswordIfUnset :one
-- First password for an account created through Telegram. The WHERE makes it
-- a no-op (no row) once any password exists: that one is changed with the
-- current password instead (POST /me/password).
UPDATE profile
SET password_hash = $2, must_change_password = false
WHERE id = $1 AND (password_hash IS NULL OR password_hash = '')
RETURNING *;

-- name: SetTelegramBotUserPendingReferral :exec
INSERT INTO telegram_bot_user (tg_user_id, pending_referral_code, pending_referral_at)
VALUES ($1, $2, now())
ON CONFLICT (tg_user_id) DO UPDATE
  SET pending_referral_code = EXCLUDED.pending_referral_code,
      pending_referral_at   = now();

-- name: GetTelegramBotUserPendingReferral :one
SELECT pending_referral_code::text AS code FROM telegram_bot_user
WHERE tg_user_id = $1
  AND pending_referral_code IS NOT NULL
  AND pending_referral_at > now() - interval '30 days';

-- name: ClearTelegramBotUserPendingReferral :exec
UPDATE telegram_bot_user
SET pending_referral_code = NULL, pending_referral_at = NULL
WHERE tg_user_id = $1;
