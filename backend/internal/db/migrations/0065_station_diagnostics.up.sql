-- Classroom PCs report what is wrong with them, so a school never has to.
--
-- Until now the only record of an agent's trouble was station.log on the
-- machine itself. With thirty PCs in a room that means walking to each one, and
-- the one time it mattered the school had already closed the window and the
-- evidence was gone.
--
-- Nothing here is student data: a station has no learner identity, and the
-- agent logs its own lifecycle only.

ALTER TABLE b2b_station
  -- The agent's own classification, mirroring internal/status.Phase:
  -- starting | ready | waiting | blocked. 'blocked' is the one that means a
  -- human has to do something, and it is what makes a fleet list useful.
  ADD COLUMN last_phase    text NOT NULL DEFAULT '',
  -- The machine-readable cause, e.g. hwid_other_org, clock, no_license.
  ADD COLUMN last_code     text NOT NULL DEFAULT '',
  -- The same thing in Uzbek, already written for a school administrator.
  ADD COLUMN last_problem  text NOT NULL DEFAULT '',
  -- How far this PC's clock is from ours, in seconds. Null when unmeasured.
  -- Broken out of the free text because a skew past two minutes makes every
  -- signature look replayed (see station_auth.go) and the resulting
  -- station_unauthorized is indistinguishable from a revoked station.
  ADD COLUMN clock_offset_seconds integer,
  ADD COLUMN last_diag_at  timestamptz;

-- station_id is NULLABLE, and that is the important part of this table.
--
-- The reports worth having most come from PCs that never became stations: the
-- machine already registered to another school, the school with no licence
-- seats left, the installer key that expired. None of those can hold a station
-- token, so none of them could file a report through an authenticated route --
-- they would be exactly the silent failures this table exists to end. Those
-- rows carry org_id and hwid_hash instead, taken from the installer key the
-- agent was trying to enrol with.
CREATE TABLE b2b_station_diag (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id        uuid NOT NULL REFERENCES b2b_org(id) ON DELETE CASCADE,
  station_id    uuid REFERENCES b2b_station(id) ON DELETE CASCADE,
  -- Identifies a machine that has no station row yet. Not a secret: every
  -- agent transmits it on every enrolment attempt.
  hwid_hash     text NOT NULL DEFAULT '',
  label         text NOT NULL DEFAULT '',
  created_at    timestamptz NOT NULL DEFAULT now(),
  agent_version text NOT NULL DEFAULT '',
  phase         text NOT NULL DEFAULT '',
  code          text NOT NULL DEFAULT '',
  problem       text NOT NULL DEFAULT '',
  detail        text NOT NULL DEFAULT '',
  clock_offset_seconds integer,
  os            text NOT NULL DEFAULT '',
  -- The tail of station.log. Bounded by the handler before it gets here; the
  -- interesting failures are all in the last few hundred lines and a whole
  -- file would buy nothing.
  log_tail      text NOT NULL DEFAULT ''
);

-- Every read is "the newest reports for one school", and the retention sweep
-- on insert is the same shape narrowed to one machine.
CREATE INDEX b2b_station_diag_org_idx
  ON b2b_station_diag (org_id, created_at DESC);
CREATE INDEX b2b_station_diag_station_idx
  ON b2b_station_diag (station_id, created_at DESC)
  WHERE station_id IS NOT NULL;
CREATE INDEX b2b_station_diag_hwid_idx
  ON b2b_station_diag (org_id, hwid_hash, created_at DESC)
  WHERE station_id IS NULL;
