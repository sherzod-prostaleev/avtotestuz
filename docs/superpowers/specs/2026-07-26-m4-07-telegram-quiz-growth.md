# M4-07 — Telegram on-demand quiz (growth / reklama)

**Date:** 2026-07-26  
**Status:** implemented  
**Depends on:** M4-06 bot foundation (`internal/bot` link + webhook/longpoll)

## Product

- **Groups + DM:** `/quiz` starts an unlimited session. Prefer questions with images (`sendPhoto` + inline answer buttons). Text-only fallback if no image or photo send fails.
- **Not** a once-daily cron poll into groups.
- After each answer: correct/wrong + soft Driver Go CTA URL button + **Keyingi savol** / **To‘xtatish**.
- Commands: `/quiz`, `/next`, `/stop`, `/start` (group help), `/link`, `/status`, `/unlink`.
- Optional ops cron: `cmd/tgdigest` soft due reminders to **linked DMs only**.

## Schema

Migration `0039_telegram_quiz_session`:

- `telegram_chat` — my_chat_member registry
- `telegram_quiz_session` — one active session per chat (partial unique index)
- Feature flags: `telegram_quiz`, `telegram_dm_digest`

## Trust

- Callbacks trust `chat_id` + DB session (`awaiting_answer`, `question_id`); answer index only. Question ID is never taken from callback payload as authority.
- Double-tap: `MarkQuizSessionAnswered` is conditional on `awaiting_answer` (`execrows`).

## Ops

```bash
make tg-digest           # dry-run
make tg-digest-send      # live DMs
```

Webhook/longpoll `allowed_updates`: `message`, `callback_query`, `my_chat_member`.
