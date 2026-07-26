-- Raise soft daily study goal so "Bugun: X/Y" matches a realistic exam-prep load.
-- Official mock is 20 Q; covering ~1231 questions needs sustained daily volume.
-- Existing streaks that still carry the old default of 10 are backfilled.
UPDATE limit_config
SET free_value = 30, vip_value = 40
WHERE key = 'daily_goal_default';

UPDATE streak
SET daily_goal = 30
WHERE daily_goal = 10;
