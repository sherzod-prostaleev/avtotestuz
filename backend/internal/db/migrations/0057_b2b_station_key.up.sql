-- Replace the copyable device-fingerprint binding with a hardware-bound
-- Ed25519 keypair, and let a station authenticate as itself via a shadow
-- profile so classroom PCs need no learner accounts.

ALTER TABLE profile
  ADD COLUMN kind text NOT NULL DEFAULT 'user'
  CHECK (kind IN ('user', 'station'));

-- Prod has no live stations; the old binding is worthless, so start clean.
TRUNCATE b2b_station, b2b_station_activate_code;

-- Drop activate_code_id (and its FK to b2b_station_activate_code) before
-- dropping that table, or the DROP TABLE fails on the dangling dependency.
ALTER TABLE b2b_station
  DROP COLUMN fingerprint,
  DROP COLUMN activate_code_id,
  ADD COLUMN public_key         bytea NOT NULL,
  ADD COLUMN hwid_hash          text  NOT NULL,
  ADD COLUMN agent_version      text  NOT NULL DEFAULT '',
  ADD COLUMN last_ip            inet,
  ADD COLUMN station_profile_id uuid REFERENCES profile(id) ON DELETE SET NULL;

DROP TABLE b2b_station_activate_code;

-- One active bind per physical machine, globally (anti-leak across orgs).
CREATE UNIQUE INDEX b2b_station_active_hwid_uidx
  ON b2b_station (hwid_hash) WHERE status = 'active';

CREATE TABLE b2b_org_enroll_code (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id     uuid NOT NULL REFERENCES b2b_org(id) ON DELETE CASCADE,
  code       text NOT NULL UNIQUE,
  max_uses   int  NOT NULL CHECK (max_uses > 0),
  used_count int  NOT NULL DEFAULT 0 CHECK (used_count >= 0),
  expires_at timestamptz NOT NULL,
  revoked_at timestamptz,
  created_by text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX b2b_org_enroll_code_org_idx
  ON b2b_org_enroll_code (org_id, expires_at DESC);
