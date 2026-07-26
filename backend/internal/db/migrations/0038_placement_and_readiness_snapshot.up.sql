-- Placement diagnostic mode + readiness snapshot at exam finish (calibration).

ALTER TABLE exam_session
  DROP CONSTRAINT IF EXISTS exam_session_mode_check;

ALTER TABLE exam_session
  ADD CONSTRAINT exam_session_mode_check
  CHECK (mode IN (
    'variant', 'exam', 'practice', 'mistakes', 'grand_mock', 'review', 'placement'
  ));

ALTER TABLE exam_session
  ADD COLUMN IF NOT EXISTS readiness_pct_at_finish integer
    CHECK (readiness_pct_at_finish IS NULL OR (readiness_pct_at_finish >= 0 AND readiness_pct_at_finish <= 100));

COMMENT ON COLUMN exam_session.readiness_pct_at_finish IS
  'Bank-honest readiness_pct captured at FinishSession for exam/grand_mock/placement calibration.';
