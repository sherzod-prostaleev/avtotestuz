-- The learner's Telegram profile photo, shown in place of the initial letter.
-- Only learners whose Telegram link Telegram itself vouched for
-- (telegram_account.phone_verified_at) get one: a legacy /start <token> link
-- can bind someone else's Telegram to a profile, and that person's face must
-- not appear on it. avatar_key is an unguessable object key in the public
-- media bucket (never derived from a profile or Telegram id). The CHECK names
-- the only source today; key and source are set and cleared together.
--
-- avatar_updated_at is when the photo was last checked with Telegram, also
-- when the check found no usable photo (key NULL): it gates the weekly
-- refresh, so a learner who hides their photo from bots is not asked about on
-- every page load. NULL = never checked.
ALTER TABLE profile
  ADD COLUMN avatar_key text,
  ADD COLUMN avatar_source text CHECK (avatar_source IN ('telegram')),
  ADD COLUMN avatar_updated_at timestamptz,
  ADD CONSTRAINT profile_avatar_key_source_together
    CHECK ((avatar_key IS NULL) = (avatar_source IS NULL));
