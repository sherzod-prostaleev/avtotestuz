-- name: CreateLinkToken :one
INSERT INTO telegram_link_token (profile_id, token_hash, expires_at)
VALUES ($1, $2, $3)
RETURNING id, expires_at;

-- name: DeleteUnusedLinkTokensForProfile :exec
-- Keeps the table to one live row per profile: a fresh generate() call makes
-- any earlier, still-unused token for the same profile immediately
-- unredeemable rather than leaving several tokens with confusingly
-- different expiries alive at once.
DELETE FROM telegram_link_token WHERE profile_id = $1 AND used_at IS NULL;

-- name: GetLinkTokenByHashForUpdate :one
-- Row-locking lookup for the redeem path (bot.LinkService.RedeemLinkToken):
-- locks the token row for the duration of the check-then-act (status check,
-- then telegram_account upsert, then mark-used) so two concurrent redeem
-- attempts for the same token serialize instead of both passing the
-- used_at IS NULL check before either writes it. Same shape as
-- LockProfileForGrant in billing.
SELECT id, profile_id, expires_at, used_at
FROM telegram_link_token
WHERE token_hash = $1
FOR UPDATE;

-- name: MarkLinkTokenUsed :exec
UPDATE telegram_link_token SET used_at = now() WHERE id = $1;

-- name: GetTelegramAccountByTgUserID :one
SELECT profile_id, tg_user_id, username, linked_at
FROM telegram_account
WHERE tg_user_id = $1;

-- name: UpsertTelegramAccount :exec
-- ON CONFLICT targets profile_id (the primary key) only: re-linking the same
-- profile to a new tg_user_id is allowed (last link wins, see design §3.3).
-- Binding a tg_user_id that already belongs to a DIFFERENT profile is not
-- caught by this ON CONFLICT clause — it instead raises a unique_violation
-- on telegram_account's tg_user_id constraint, which the caller must catch
-- and translate to ErrTelegramAccountLinkedElsewhere. That's deliberate: it
-- is the atomic backstop against a race between two concurrent redemptions
-- targeting the same Telegram account for two different profiles.
INSERT INTO telegram_account (profile_id, tg_user_id, username, linked_at)
VALUES ($1, $2, $3, now())
ON CONFLICT (profile_id) DO UPDATE SET
  tg_user_id = EXCLUDED.tg_user_id,
  username   = EXCLUDED.username,
  linked_at  = now();
