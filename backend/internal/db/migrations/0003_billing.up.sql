CREATE TABLE tariff (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  code          text NOT NULL UNIQUE,
  days          int  NOT NULL CHECK (days > 0),
  price_uzs     bigint NOT NULL CHECK (price_uzs >= 0),
  old_price_uzs bigint,
  badge         text,
  sort_order    int NOT NULL DEFAULT 0,
  active        boolean NOT NULL DEFAULT true
);

CREATE TABLE tariff_translation (
  tariff_id   uuid NOT NULL REFERENCES tariff(id) ON DELETE CASCADE,
  locale      locale_code NOT NULL,
  name        text NOT NULL,
  description text NOT NULL DEFAULT '',
  PRIMARY KEY (tariff_id, locale)
);

CREATE TABLE promo_code (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  code           text NOT NULL UNIQUE,
  kind           text NOT NULL CHECK (kind IN ('percent','fixed','days')),
  value          int NOT NULL CHECK (value > 0),
  max_uses       int,
  per_user_limit int NOT NULL DEFAULT 1,
  valid_from     timestamptz,
  valid_to       timestamptz,
  active         boolean NOT NULL DEFAULT true,
  created_by     uuid REFERENCES profile(id)
);

CREATE TABLE payment (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  profile_id      uuid NOT NULL REFERENCES profile(id),
  tariff_id       uuid NOT NULL REFERENCES tariff(id),
  amount_uzs      bigint NOT NULL CHECK (amount_uzs >= 0),
  provider        text NOT NULL CHECK (provider IN ('payme','click','sandbox')),
  status          text NOT NULL DEFAULT 'created'
                  CHECK (status IN ('created','pending','paid','failed','canceled','refunded')),
  provider_txn_id text,
  idempotency_key text NOT NULL UNIQUE,
  promo_code_id   uuid REFERENCES promo_code(id),
  meta            jsonb NOT NULL DEFAULT '{}',
  created_at      timestamptz NOT NULL DEFAULT now(),
  paid_at         timestamptz
);
CREATE INDEX payment_profile_idx ON payment(profile_id, created_at DESC);

CREATE TABLE entitlement (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  profile_id uuid NOT NULL REFERENCES profile(id) ON DELETE CASCADE,
  source     text NOT NULL CHECK (source IN ('purchase','promo','referral','admin','b2b')),
  starts_at  timestamptz NOT NULL,
  ends_at    timestamptz NOT NULL,
  payment_id uuid REFERENCES payment(id),
  created_by uuid REFERENCES profile(id),
  note       text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  CHECK (ends_at > starts_at)
);
CREATE INDEX entitlement_profile_idx ON entitlement(profile_id, ends_at DESC);

CREATE TABLE promo_redemption (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  promo_code_id uuid NOT NULL REFERENCES promo_code(id),
  profile_id    uuid NOT NULL REFERENCES profile(id),
  payment_id    uuid REFERENCES payment(id),
  created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE referral_attribution (
  referee_id    uuid PRIMARY KEY REFERENCES profile(id) ON DELETE CASCADE,
  referrer_id   uuid NOT NULL REFERENCES profile(id) ON DELETE CASCADE,
  reward_status text NOT NULL DEFAULT 'pending'
                CHECK (reward_status IN ('pending','granted','rejected')),
  fraud_flags   jsonb NOT NULL DEFAULT '{}',
  created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE limit_config (
  key        text PRIMARY KEY,
  free_value int NOT NULL,
  vip_value  int NOT NULL,
  updated_at timestamptz NOT NULL DEFAULT now(),
  -- informational only (audit_log is the source of truth); no FK so that
  -- TRUNCATE ... CASCADE on profile never wipes seeded config
  updated_by uuid
);

-- -1 = cheksiz (unlimited)
INSERT INTO limit_config (key, free_value, vip_value) VALUES
  ('daily_practice_questions', 10, -1),
  ('unlock_threshold_correct', 10, 10),
  ('grand_mock_threshold_pct', 85, 85),
  ('daily_goal_default',       10, 10);
