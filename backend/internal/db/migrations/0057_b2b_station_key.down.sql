DROP TABLE IF EXISTS b2b_org_enroll_code;
DROP INDEX IF EXISTS b2b_station_active_hwid_uidx;

TRUNCATE b2b_station;
ALTER TABLE b2b_station
  DROP COLUMN station_profile_id,
  DROP COLUMN last_ip,
  DROP COLUMN agent_version,
  DROP COLUMN hwid_hash,
  DROP COLUMN public_key,
  ADD COLUMN fingerprint text NOT NULL,
  ADD COLUMN activate_code_id uuid;

CREATE UNIQUE INDEX b2b_station_active_fingerprint_uidx
  ON b2b_station (fingerprint) WHERE status = 'active';

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

ALTER TABLE profile DROP COLUMN kind;
