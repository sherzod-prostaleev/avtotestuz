-- Spaced-repetition review sessions (FSRS due queue via GET /learn/next /
-- session mode=review). Distinct from VIP mistakes bank (lapses > 0).
ALTER TABLE exam_session
  DROP CONSTRAINT IF EXISTS exam_session_mode_check;

ALTER TABLE exam_session
  ADD CONSTRAINT exam_session_mode_check
  CHECK (mode IN ('variant', 'exam', 'practice', 'mistakes', 'grand_mock', 'review'));
