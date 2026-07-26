UPDATE limit_config
SET free_value = 10, vip_value = 10
WHERE key = 'daily_goal_default';

-- Do not force streaks back to 10 — learners may have raised goals intentionally.
-- Down migration only restores the config seed default.
