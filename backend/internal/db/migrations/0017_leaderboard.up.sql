-- Partial index for RebuildPeriod's per-profile correct-answer aggregation
-- (see internal/leaderboard.Service.RebuildPeriod) — without it, that
-- GROUP BY becomes a full table scan as session_answer grows. Only correct
-- answers are ever queried by that path, hence the partial WHERE.
CREATE INDEX session_answer_correct_answered_idx
  ON session_answer(answered_at) WHERE is_correct;

-- Daily leaderboard point cap: even VIP users are capped so ranking
-- reflects consistent effort rather than raw available time. See
-- docs/superpowers/specs/2026-07-25-m4-01-leaderboard-design.md section 4.
INSERT INTO limit_config (key, free_value, vip_value) VALUES
  ('leaderboard_daily_points', 30, 100);
