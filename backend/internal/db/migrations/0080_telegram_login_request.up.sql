-- «Telegram orqali kirish» from the website (.superpowers/sdd/tglogin-brief.md).
--
-- A browser starts a request and gets a t.me deep link carrying `token`; the
-- learner opens it in the bot, proves who they are there (a phone-verified
-- link + «✅ Kirish», or a share of their own phone number), and the waiting
-- browser then trades the approved request for a session. Every secret is
-- stored as a sha256 digest only: the raw token lives in the deep link, the
-- raw browser secret in an HttpOnly cookie of the browser that started it,
-- and the raw confirm nonce in the bot buttons' callback_data. Completing
-- needs the token AND the browser secret, so a leaked link alone signs no
-- one in anywhere but the browser that asked.
CREATE TABLE telegram_login_request (
  id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  token_hash          text NOT NULL UNIQUE,
  browser_secret_hash text NOT NULL,
  -- Coarse "Chrome · Android" built server-side from a fixed vocabulary
  -- (auth.DescribeDevice), shown in the bot prompt. Never the IP.
  device              text NOT NULL DEFAULT '',
  status              text NOT NULL DEFAULT 'pending'
                      CHECK (status IN ('pending', 'approved', 'cancelled', 'blocked', 'consumed')),
  -- The Telegram user whose bot chat currently owns the next step. One live
  -- pending request per Telegram user (index below), so a shared contact
  -- answers exactly one request.
  pending_tg_user_id  bigint,
  -- NULL = no «✅ Kirish» question outstanding.
  confirm_nonce_hash  text,
  -- The learner the request signs in to; set exactly when approved.
  profile_id          uuid REFERENCES profile(id) ON DELETE CASCADE,
  approved_tg_user_id bigint,
  expires_at          timestamptz NOT NULL,
  approved_at         timestamptz,
  consumed_at         timestamptz,
  created_at          timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT telegram_login_request_profile_when_approved
    CHECK ((status IN ('approved', 'consumed')) = (profile_id IS NOT NULL))
);

CREATE UNIQUE INDEX telegram_login_request_confirm_nonce_idx
  ON telegram_login_request (confirm_nonce_hash)
  WHERE confirm_nonce_hash IS NOT NULL;

CREATE UNIQUE INDEX telegram_login_request_pending_tg_idx
  ON telegram_login_request (pending_tg_user_id)
  WHERE pending_tg_user_id IS NOT NULL AND status = 'pending';

-- Retention sweep (auth.StartTelegramLogin deletes rows a day past expiry).
CREATE INDEX telegram_login_request_expires_idx ON telegram_login_request (expires_at);

-- A referral code that reached the bot as /start ref_<CODE>, waiting for this
-- Telegram user's first profile (Telegram login or the Mini App phone share).
-- Applied on profile creation only, and only within 30 days of being set.
ALTER TABLE telegram_bot_user
  ADD COLUMN pending_referral_code text,
  ADD COLUMN pending_referral_at   timestamptz,
  ADD CONSTRAINT telegram_bot_user_pending_referral_code_chk
    CHECK (pending_referral_code IS NULL OR pending_referral_code ~ '^[A-Za-z0-9_-]{1,64}$');
