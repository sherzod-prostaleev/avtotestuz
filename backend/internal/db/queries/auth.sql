-- name: CreateOTPChallenge :one
INSERT INTO otp_challenge (phone, code_hash, channel, expires_at)
VALUES ($1, $2, $3, $4) RETURNING id;

-- name: LatestOTPChallenge :one
SELECT * FROM otp_challenge
WHERE phone = $1 AND consumed = false
ORDER BY created_at DESC LIMIT 1;

-- name: IncrementOTPAttempts :exec
UPDATE otp_challenge SET attempts = attempts + 1 WHERE id = $1;

-- name: ConsumeOTP :exec
UPDATE otp_challenge SET consumed = true WHERE id = $1;

-- name: GetProfileByPhone :one
SELECT * FROM profile WHERE phone = $1;

-- name: GetProfileByID :one
SELECT * FROM profile WHERE id = $1;

-- name: GetProfileKind :one
-- Used by leaderboard.Service.RecordPoint to keep station shadow profiles
-- (kind = 'station') out of the live leaderboard write path before it ever
-- touches Billing.Status or Redis. Selecting just the column, rather than
-- reusing GetProfileByID, keeps the hot answer-submission path from paying
-- for columns it doesn't need.
SELECT kind FROM profile WHERE id = $1;

-- name: SetBypassVariantProgress :exec
UPDATE profile SET bypass_variant_progress = $2 WHERE id = $1;

-- name: CreateProfile :one
INSERT INTO profile (phone, referral_code, password_hash, name)
VALUES ($1, $2, $3, $4) RETURNING *;

-- name: SetPasswordHashIfNull :execrows
UPDATE profile
SET password_hash = $2
WHERE phone = $1 AND password_hash IS NULL;

-- name: SetProfilePassword :one
-- Replaces password_hash and clears/sets the must-change flag.
-- Never stores plaintext; callers pass a bcrypt hash only.
UPDATE profile
SET password_hash = $2,
    must_change_password = $3
WHERE id = $1
RETURNING *;

-- name: UpdateProfileMe :one
UPDATE profile SET
  name = $2, region = $3, district = $4, birth_date = $5,
  locale_pref = $6, theme_pref = $7
WHERE id = $1 RETURNING *;

-- name: CreateRefreshToken :exec
INSERT INTO refresh_token (profile_id, token_hash, expires_at) VALUES ($1, $2, $3);

-- name: GetRefreshToken :one
SELECT * FROM refresh_token WHERE token_hash = $1;

-- name: RevokeRefreshToken :exec
UPDATE refresh_token SET revoked_at = now() WHERE id = $1 AND revoked_at IS NULL;

-- name: DeleteRefreshToken :exec
DELETE FROM refresh_token WHERE id = $1;

-- name: RevokeAllRefreshTokens :exec
UPDATE refresh_token SET revoked_at = now() WHERE profile_id = $1 AND revoked_at IS NULL;

-- name: ActiveEntitlementEnd :one
SELECT ends_at FROM entitlement
WHERE profile_id = $1 AND ends_at > now()
ORDER BY ends_at DESC LIMIT 1;

-- name: InsertEntitlement :one
INSERT INTO entitlement (profile_id, source, starts_at, ends_at, note, created_by, payment_id)
VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id;

-- name: GetLatestPurchaseEntitlement :one
-- Most recent purchase-sourced grant for learner-facing proration messaging.
SELECT id, profile_id, source, starts_at, ends_at, payment_id, created_by, note, created_at
FROM entitlement
WHERE profile_id = $1 AND source = 'purchase'
ORDER BY created_at DESC
LIMIT 1;

-- name: GetEntitlementByPaymentID :one
SELECT id, profile_id, source, starts_at, ends_at, payment_id, created_by, note, created_at
FROM entitlement
WHERE payment_id = $1
LIMIT 1;

-- name: ClampEntitlementEnd :exec
UPDATE entitlement
SET ends_at = $2,
    note = note || $3
WHERE id = $1 AND ends_at > $2;

-- name: ClampAllEntitlementsForPayment :exec
-- Clamps purchase + referral_buyer rows for one payment (refund path).
UPDATE entitlement
SET ends_at = CASE
      WHEN starts_at >= sqlc.arg(clamp_to)::timestamptz THEN starts_at + interval '1 second'
      ELSE sqlc.arg(clamp_to)::timestamptz
    END,
    note = note || sqlc.arg(note_suffix)
WHERE payment_id = sqlc.arg(payment_id) AND ends_at > sqlc.arg(clamp_to)::timestamptz;


