# U-10 TG daily quiz — closed (M4-07 shipped)

**Date:** 2026-07-26  
**Decision:** **Done** — on-demand group/DM quiz + optional DM digest (not inventing a daily group cron).

## Shipped

- On-demand `/quiz` sessions (image-first `sendPhoto` + inline answers)
- `/next`, `/stop`, `/unlink`, group `/start` help
- `telegram_chat` + `telegram_quiz_session` (mig `0039`)
- Soft CTA on every graded answer
- Optional `cmd/tgdigest` for linked-user due reminders

## Explicitly out of scope (still)

- Multi-locale bot copy
- Group leaderboard
- Auto-spam cron into groups
- Admin web UI for chat list

See `2026-07-26-m4-07-telegram-quiz-growth.md`.
