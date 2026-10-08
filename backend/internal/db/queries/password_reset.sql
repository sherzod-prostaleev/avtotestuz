-- name: CreatePasswordResetToken :one
INSERT INTO password_reset_token (profile_id, token_hash, expires_at)
VALUES ($1, $2, $3)
RETURNING id, expires_at;

-- name: DeleteUnusedPasswordResetTokensForProfile :exec
DELETE FROM password_reset_token
WHERE profile_id = $1 AND used_at IS NULL;

-- name: GetPasswordResetTokenByHash :one
SELECT id, profile_id, token_hash, expires_at, used_at, verified_at, pending_tg_user_id, created_at, confirm_nonce_hash, verified_tg_user_id
FROM password_reset_token
WHERE token_hash = $1;

-- name: GetPasswordResetTokenByHashForUpdate :one
SELECT id, profile_id, token_hash, expires_at, used_at, verified_at, pending_tg_user_id, created_at, confirm_nonce_hash, verified_tg_user_id
FROM password_reset_token
WHERE token_hash = $1
FOR UPDATE;

-- name: GetLivePasswordResetByPendingTgForUpdate :one
SELECT id, profile_id, token_hash, expires_at, used_at, verified_at, pending_tg_user_id, created_at, confirm_nonce_hash, verified_tg_user_id
FROM password_reset_token
WHERE pending_tg_user_id = $1 AND used_at IS NULL
ORDER BY created_at DESC
LIMIT 1
FOR UPDATE;

-- name: ClearPasswordResetPendingForTg :exec
UPDATE password_reset_token
SET pending_tg_user_id = NULL,
    confirm_nonce_hash = NULL
WHERE pending_tg_user_id = $1 AND used_at IS NULL AND id <> $2;

-- name: ClearAllPasswordResetPendingForTg :exec
UPDATE password_reset_token
SET pending_tg_user_id = NULL,
    confirm_nonce_hash = NULL
WHERE pending_tg_user_id = $1 AND used_at IS NULL;

-- name: SetPasswordResetPendingTg :exec
-- A new /start re-arms the reset for this Telegram user: any confirm question
-- asked before it is void.
UPDATE password_reset_token
SET pending_tg_user_id = $2,
    confirm_nonce_hash = NULL
WHERE id = $1 AND used_at IS NULL;

-- name: SetPasswordResetConfirmNonce :exec
UPDATE password_reset_token
SET confirm_nonce_hash = $2
WHERE id = $1 AND used_at IS NULL AND verified_at IS NULL;

-- name: GetPasswordResetByConfirmNonceForUpdate :one
SELECT id, profile_id, token_hash, expires_at, used_at, verified_at, pending_tg_user_id, created_at, confirm_nonce_hash, verified_tg_user_id
FROM password_reset_token
WHERE confirm_nonce_hash = $1
FOR UPDATE;

-- name: CancelPasswordReset :exec
-- «Yo'q» in the bot: the reset is spent, so the website tab sees "invalid"
-- and CompletePasswordReset refuses it.
UPDATE password_reset_token
SET used_at = now(),
    pending_tg_user_id = NULL,
    confirm_nonce_hash = NULL
WHERE id = $1 AND used_at IS NULL;

-- name: MarkPasswordResetVerified :exec
-- The right-hand side reads the pre-update row, so the confirming Telegram
-- user moves from pending_tg_user_id (whose unique index must free up) into
-- verified_tg_user_id for CompletePasswordReset.
UPDATE password_reset_token
SET verified_at = now(),
    verified_tg_user_id = pending_tg_user_id,
    pending_tg_user_id = NULL,
    confirm_nonce_hash = NULL
WHERE id = $1 AND used_at IS NULL;

-- name: MarkPasswordResetUsed :exec
UPDATE password_reset_token
SET used_at = now(),
    pending_tg_user_id = NULL,
    confirm_nonce_hash = NULL
WHERE id = $1;
