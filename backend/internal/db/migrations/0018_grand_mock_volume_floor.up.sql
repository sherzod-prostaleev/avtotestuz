-- Grand Mock volume floor.
--
-- grand_mock_threshold_pct (seeded in 0003) is an accuracy ratio
-- (correct/seen, weighted by category size — see learning.Service.Stats), so
-- on its own it is trivially satisfiable: answering ONE question correctly in
-- every category yields readiness_pct = 100 and unlocks a feature that hands
-- out a completion certificate. This row adds the missing second dimension —
-- how much of the question bank the profile has actually studied, counted as
-- distinct rows in question_memory so re-answering one easy question cannot
-- inflate it.
--
-- Expressed as a PERCENT of the valid question bank rather than an absolute
-- count, so it keeps meaning the same thing as content grows (25% of today's
-- ~1231 valid questions is ~308, roughly 15 biletlar worth of study) instead
-- of silently becoming trivial.
--
-- free_value == vip_value because Grand Mock is VIP-gated anyway: the VIP
-- check runs before this one, so a separate free tier would be dead.
INSERT INTO limit_config (key, free_value, vip_value) VALUES
  ('grand_mock_min_studied_pct', 25, 25);
