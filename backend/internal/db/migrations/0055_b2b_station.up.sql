-- B2B classroom stations (device-bound VIP) + optional home seats / partner promo.

ALTER TABLE b2b_org_license
  ADD COLUMN IF NOT EXISTS home_seats int NOT NULL DEFAULT 0
  CHECK (home_seats >= 0);

ALTER TABLE promo_code
  ADD COLUMN IF NOT EXISTS partner_org_id uuid REFERENCES b2b_org(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS promo_code_partner_org_idx ON promo_code (partner_org_id)
  WHERE partner_org_id IS NOT NULL;

CREATE TABLE b2b_station_activate_code (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id     uuid NOT NULL REFERENCES b2b_org(id) ON DELETE CASCADE,
  code       text NOT NULL UNIQUE,
  label      text NOT NULL DEFAULT '',
  expires_at timestamptz NOT NULL,
  used_at    timestamptz,
  created_by text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX b2b_station_activate_code_org_idx
  ON b2b_station_activate_code (org_id, expires_at DESC);

CREATE TABLE b2b_station (
  id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id           uuid NOT NULL REFERENCES b2b_org(id) ON DELETE CASCADE,
  fingerprint      text NOT NULL,
  label            text NOT NULL DEFAULT '',
  status           text NOT NULL DEFAULT 'active'
                   CHECK (status IN ('active', 'revoked')),
  activated_at     timestamptz NOT NULL DEFAULT now(),
  last_seen_at     timestamptz NOT NULL DEFAULT now(),
  activated_by     text NOT NULL DEFAULT '',
  activate_code_id uuid REFERENCES b2b_station_activate_code(id) ON DELETE SET NULL,
  created_at       timestamptz NOT NULL DEFAULT now()
);

-- One active bind per fingerprint globally (anti-leak across orgs).
CREATE UNIQUE INDEX b2b_station_active_fingerprint_uidx
  ON b2b_station (fingerprint)
  WHERE status = 'active';

CREATE INDEX b2b_station_org_idx ON b2b_station (org_id, status);
