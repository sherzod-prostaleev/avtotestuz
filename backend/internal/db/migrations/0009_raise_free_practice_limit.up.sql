-- The free daily practice allowance was 10, which made the question-count
-- selector dishonest: picking 20 or 30 silently produced a shorter session
-- with no explanation. Raising it to 30 lets the smallest offered size be
-- fully deliverable on a free account, so the paywall is reached by an
-- explicit message rather than by a session quietly shrinking.
UPDATE limit_config SET free_value = 30 WHERE key = 'daily_practice_questions';
