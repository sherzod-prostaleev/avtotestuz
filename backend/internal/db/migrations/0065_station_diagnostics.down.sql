DROP TABLE IF EXISTS b2b_station_diag;

ALTER TABLE b2b_station
  DROP COLUMN IF EXISTS last_phase,
  DROP COLUMN IF EXISTS last_code,
  DROP COLUMN IF EXISTS last_problem,
  DROP COLUMN IF EXISTS clock_offset_seconds,
  DROP COLUMN IF EXISTS last_diag_at;
