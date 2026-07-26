ALTER TABLE exam_session
  DROP COLUMN IF EXISTS readiness_pct_at_finish;

ALTER TABLE exam_session
  DROP CONSTRAINT IF EXISTS exam_session_mode_check;

ALTER TABLE exam_session
  ADD CONSTRAINT exam_session_mode_check
  CHECK (mode IN ('variant', 'exam', 'practice', 'mistakes', 'grand_mock', 'review'));
