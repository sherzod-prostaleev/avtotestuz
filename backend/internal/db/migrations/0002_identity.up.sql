CREATE TABLE profile (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  phone         text NOT NULL UNIQUE,
  name          text NOT NULL DEFAULT '',
  region        text NOT NULL DEFAULT '',
  district      text NOT NULL DEFAULT '',
  birth_date    date,
  locale_pref   locale_code NOT NULL DEFAULT 'uz-Latn',
  theme_pref    text NOT NULL DEFAULT 'dark' CHECK (theme_pref IN ('dark','light')),
  role          text NOT NULL DEFAULT 'user'
                CHECK (role IN ('user','editor','admin','superadmin')),
  referral_code text UNIQUE,
  referred_by   uuid REFERENCES profile(id),
  status        text NOT NULL DEFAULT 'active' CHECK (status IN ('active','banned')),
  created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE otp_challenge (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  phone      text NOT NULL,
  code_hash  text NOT NULL,
  channel    text NOT NULL CHECK (channel IN ('telegram','sms','sandbox')),
  expires_at timestamptz NOT NULL,
  attempts   smallint NOT NULL DEFAULT 0,
  consumed   boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX otp_phone_idx ON otp_challenge(phone, created_at DESC);

CREATE TABLE telegram_account (
  profile_id uuid PRIMARY KEY REFERENCES profile(id) ON DELETE CASCADE,
  tg_user_id bigint NOT NULL UNIQUE,
  username   text NOT NULL DEFAULT '',
  linked_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE device (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  profile_id  uuid NOT NULL REFERENCES profile(id) ON DELETE CASCADE,
  fingerprint text NOT NULL,
  first_seen  timestamptz NOT NULL DEFAULT now(),
  last_seen   timestamptz NOT NULL DEFAULT now(),
  UNIQUE (profile_id, fingerprint)
);

CREATE TABLE refresh_token (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  profile_id uuid NOT NULL REFERENCES profile(id) ON DELETE CASCADE,
  token_hash text NOT NULL UNIQUE,
  expires_at timestamptz NOT NULL,
  revoked_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE explanation_feedback (
  profile_id     uuid NOT NULL REFERENCES profile(id) ON DELETE CASCADE,
  explanation_id uuid NOT NULL REFERENCES explanation(id) ON DELETE CASCADE,
  helpful        boolean NOT NULL,
  created_at     timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (profile_id, explanation_id)
);
