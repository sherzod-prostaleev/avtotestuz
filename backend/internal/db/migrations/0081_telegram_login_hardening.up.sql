-- Telegram login hardening after the independent audit
-- (.superpowers/sdd/tglogin-audit.md, fixes in tglogin-fix-report.md).

ALTER TABLE telegram_login_request
  -- First opener wins: the Telegram user who first opened the link in the bot.
  -- Set once and never cleared, so whoever else sees the link or its QR code
  -- (a classroom screen) cannot take the request over and sign the waiting
  -- browser in to their own account.
  ADD COLUMN opened_tg_user_id bigint,
  -- The opener's own number, as Telegram vouched for it, kept between the
  -- phone share and the «✅ Kirish» tap: a share alone never approves, because
  -- the same contact message is what a password reset and the Mini App's
  -- phone sheet produce. Cleared as soon as the request leaves that step.
  ADD COLUMN contact_phone text,
  ADD CONSTRAINT telegram_login_request_contact_phone_chk
    CHECK (contact_phone IS NULL OR contact_phone ~ '^\+998[0-9]{9}$');

-- profile(id) ON DELETE CASCADE scanned the whole table per deleted profile.
CREATE INDEX telegram_login_request_profile_idx ON telegram_login_request (profile_id);

-- Kill switch of its own: TELEGRAM_BOT_USERNAME also powers the password
-- reset and referral links, so it cannot be the way to turn this off. Gates
-- the website «Telegram orqali kirish», the bot's login approvals and the
-- Mini App one-tap phone sign-in. Starts ON.
INSERT INTO feature_flag (key, type, value_json, description) VALUES
  ('telegram_login', 'boolean', 'true'::jsonb,
   'Sign-in/sign-up through Telegram: website «Telegram orqali kirish», bot approvals, Mini App one-tap phone. OFF = phone + password only')
ON CONFLICT (key) DO NOTHING;
