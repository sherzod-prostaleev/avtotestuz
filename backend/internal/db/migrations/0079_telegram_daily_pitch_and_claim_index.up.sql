-- Daily reminder fix round 1 (.superpowers/sdd/daily-report.md).
--
-- A new migration rather than an edit of 0078: 0078 never reached prod, but it
-- is already applied in every persisted per-package test database (and any
-- dev database migrated for the dry-run), and golang-migrate only tracks the
-- version number — an edited 0078 would silently never run there.

-- Tashkent day the signup pitch was last delivered to this (unlinked) user.
-- The pitch goes out at most once per 7 days; other evenings get a neutral
-- line, so an existing learner who never linked Telegram is not told to sign
-- up every single day.
ALTER TABLE telegram_bot_user ADD COLUMN last_signup_pitch_on date;

-- The claim (ClaimTelegramReminderRecipient) asks for the first eligible
-- user not yet claimed today. Ordered by (last_reminder_on NULLS FIRST,
-- tg_user_id), unclaimed users sit at the front of this index and the ones
-- claimed today at the back, so each claim reads one entry instead of
-- skipping past everyone already claimed (which made a run O(N²)).
CREATE INDEX telegram_bot_user_reminder_claim_idx
  ON telegram_bot_user (last_reminder_on NULLS FIRST, tg_user_id)
  WHERE reminders_enabled AND blocked_at IS NULL;
