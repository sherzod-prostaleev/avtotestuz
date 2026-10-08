-- name: GetProfileAvatarState :one
-- Everything the avatar code decides on, read in one round trip: the stored
-- photo, when it was last checked, and the Telegram user of the profile's
-- phone-verified link (NULL = none, so no photo may be shown or fetched).
SELECT p.kind, p.avatar_key, p.avatar_updated_at,
       ta.tg_user_id AS verified_tg_user_id
FROM profile p
LEFT JOIN telegram_account ta
       ON ta.profile_id = p.id AND ta.phone_verified_at IS NOT NULL
WHERE p.id = sqlc.arg(profile_id);

-- name: SetProfileTelegramAvatar :one
-- Records a finished Telegram check (avatar_key NULL = no usable photo) and
-- returns the key it replaced, for the caller to delete. It only applies
-- while the profile is a learner whose phone-verified link still points at
-- the Telegram user the photo came from: a fetch that raced an unlink or a
-- re-link writes nothing (no row), and the caller drops the object it just
-- uploaded. The CTE locks the row so the returned previous key is the one
-- actually overwritten.
WITH old AS (
  SELECT o.id, o.avatar_key FROM profile o WHERE o.id = sqlc.arg(profile_id) FOR UPDATE
)
UPDATE profile p
SET avatar_key = sqlc.narg(avatar_key)::text,
    avatar_source = CASE WHEN sqlc.narg(avatar_key)::text IS NULL THEN NULL ELSE 'telegram' END,
    avatar_updated_at = now()
FROM old
WHERE p.id = old.id
  AND p.kind = 'user'
  AND EXISTS (
    SELECT 1 FROM telegram_account ta
    WHERE ta.profile_id = p.id
      AND ta.tg_user_id = sqlc.arg(tg_user_id)
      AND ta.phone_verified_at IS NOT NULL
  )
RETURNING old.avatar_key AS previous_key;

-- name: ClearProfileAvatarUnlessVerified :one
-- Drops the photo of a profile that no longer has a phone-verified link (or
-- is not a learner). Judged in the same statement as the write, so a link
-- re-proven concurrently keeps its photo. No row = nothing to clear.
WITH old AS (
  SELECT o.id, o.avatar_key FROM profile o WHERE o.id = sqlc.arg(profile_id) FOR UPDATE
)
UPDATE profile p
SET avatar_key = NULL, avatar_source = NULL, avatar_updated_at = NULL
FROM old
WHERE p.id = old.id
  AND (p.avatar_key IS NOT NULL OR p.avatar_updated_at IS NOT NULL)
  AND (p.kind <> 'user' OR NOT EXISTS (
    SELECT 1 FROM telegram_account ta
    WHERE ta.profile_id = p.id AND ta.phone_verified_at IS NOT NULL
  ))
RETURNING old.avatar_key AS previous_key;
