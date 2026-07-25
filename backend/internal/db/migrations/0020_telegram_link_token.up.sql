-- One-time, expiring tokens that bind a link attempt to the profile that
-- generated it (authenticated web session). The Telegram bot redeems these
-- in-process; there is deliberately no HTTP endpoint that accepts a
-- profile_id/tg_user_id pair directly — see
-- docs/superpowers/specs/2026-07-25-m4-06-telegram-bot-design.md §3.
CREATE TABLE telegram_link_token (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  profile_id uuid NOT NULL REFERENCES profile(id) ON DELETE CASCADE,
  token_hash text NOT NULL UNIQUE,
  expires_at timestamptz NOT NULL,
  used_at    timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX telegram_link_token_profile_idx ON telegram_link_token(profile_id);
