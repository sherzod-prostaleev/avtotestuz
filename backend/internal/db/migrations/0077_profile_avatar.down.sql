ALTER TABLE profile
  DROP CONSTRAINT IF EXISTS profile_avatar_key_source_together,
  DROP COLUMN IF EXISTS avatar_updated_at,
  DROP COLUMN IF EXISTS avatar_source,
  DROP COLUMN IF EXISTS avatar_key;
