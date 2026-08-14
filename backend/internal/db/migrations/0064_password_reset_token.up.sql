-- Learner password reset: identity is proven in Telegram (linked account or
-- matching contact), then a new password is set on the website. Tokens are
-- stored hashed; the raw value lives only in the t.me deep link and the
-- subsequent /reset-password URL. See auth.StartPasswordReset.
CREATE TABLE password_reset_token (
  id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  profile_id         uuid NOT NULL REFERENCES profile(id) ON DELETE CASCADE,
  token_hash         text NOT NULL UNIQUE,
  expires_at         timestamptz NOT NULL,
  used_at            timestamptz,
  verified_at        timestamptz,
  pending_tg_user_id bigint,
  created_at         timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX password_reset_token_profile_live_idx
  ON password_reset_token (profile_id)
  WHERE used_at IS NULL;

CREATE UNIQUE INDEX password_reset_token_pending_tg_live_idx
  ON password_reset_token (pending_tg_user_id)
  WHERE pending_tg_user_id IS NOT NULL AND used_at IS NULL;
