-- name: CreatePasswordResetToken :one
INSERT INTO password_reset_token (profile_id, token_hash, expires_at)
VALUES ($1, $2, $3)
RETURNING id, expires_at;

-- name: DeleteUnusedPasswordResetTokensForProfile :exec
DELETE FROM password_reset_token
WHERE profile_id = $1 AND used_at IS NULL;

-- name: GetPasswordResetTokenByHash :one
SELECT id, profile_id, token_hash, expires_at, used_at, verified_at, pending_tg_user_id, created_at
FROM password_reset_token
WHERE token_hash = $1;

-- name: GetPasswordResetTokenByHashForUpdate :one
SELECT id, profile_id, token_hash, expires_at, used_at, verified_at, pending_tg_user_id, created_at
FROM password_reset_token
WHERE token_hash = $1
FOR UPDATE;

-- name: GetLivePasswordResetByPendingTgForUpdate :one
SELECT id, profile_id, token_hash, expires_at, used_at, verified_at, pending_tg_user_id, created_at
FROM password_reset_token
WHERE pending_tg_user_id = $1 AND used_at IS NULL
ORDER BY created_at DESC
LIMIT 1
FOR UPDATE;

-- name: ClearPasswordResetPendingForTg :exec
UPDATE password_reset_token
SET pending_tg_user_id = NULL
WHERE pending_tg_user_id = $1 AND used_at IS NULL AND id <> $2;

-- name: SetPasswordResetPendingTg :exec
UPDATE password_reset_token
SET pending_tg_user_id = $2
WHERE id = $1 AND used_at IS NULL;

-- name: MarkPasswordResetVerified :exec
UPDATE password_reset_token
SET verified_at = now(),
    pending_tg_user_id = NULL
WHERE id = $1 AND used_at IS NULL;

-- name: MarkPasswordResetUsed :exec
UPDATE password_reset_token
SET used_at = now(),
    pending_tg_user_id = NULL
WHERE id = $1;
