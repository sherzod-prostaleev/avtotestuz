# Driver Go — M4-08 Web Push Foundation (U-11)

**Date:** 2026-07-26  
**Status:** Foundation slice (subscribe + send path). Campaigns / admin broadcast = later (M3 Support → Broadcasts).  
**Depends on:** Auth JWT. Independent of M4-07 TG quiz.

---

## 1. Scope (this slice)

| In | Out |
|----|-----|
| `push_subscription` table | Admin campaign UI / targeting |
| VAPID keys via env (optional; empty = disabled) | Cron / scheduled digests |
| `GET/POST/DELETE /me/push*` learner API | Multi-locale push templates engine |
| Minimal service worker (show notification on `push`) | Full PWA offline cache (M6 / U-38–39) |
| Profile card: enable / disable | Arena duel invite pushes (follow-up) |
| `POST /me/push/test` self-smoke when configured | SMS / Telegram channel senders |

`notification.channel` gains `'webpush'`. Rows are written when a push is attempted (audit trail even if delivery fails).

---

## 2. Locked decisions

1. **VAPID optional.** Empty keys → API returns `configured: false`; subscribe rejected with `web_push_unconfigured`. Same pattern as Telegram bot username.
2. **Endpoint uniqueness.** One browser push endpoint maps to one row; re-subscribe upserts keys + `profile_id` (device can move accounts).
3. **Sender interface.** Production uses `webpush-go`; tests inject a fake. Gone/expired (410) → delete subscription.
4. **SW is push-only.** No precache / offline shell here — that stays M6.
5. **No invented keys in repo.** Operators generate VAPID locally (`npx web-push generate-vapid-keys` or equivalent).

---

## 3. API

| Method | Path | Auth | Notes |
|--------|------|------|-------|
| GET | `/api/v1/me/push` | JWT | `{ configured, subscribed, subscription_count, vapid_public_key? }` |
| POST | `/api/v1/me/push/subscribe` | JWT | body: `{ endpoint, keys: { p256dh, auth }, user_agent? }` |
| DELETE | `/api/v1/me/push/subscribe` | JWT | body: `{ endpoint }` — drop this browser |
| POST | `/api/v1/me/push/test` | JWT | sends a tiny self-test if configured + ≥1 sub |

---

## 4. Schema

```sql
push_subscription(
  id uuid PK,
  profile_id FK → profile ON DELETE CASCADE,
  endpoint text UNIQUE NOT NULL,
  p256dh text NOT NULL,
  auth text NOT NULL,
  user_agent text NOT NULL DEFAULT '',
  created_at timestamptz,
  last_seen timestamptz
)
```

---

## 5. Env

```
VAPID_PUBLIC_KEY=
VAPID_PRIVATE_KEY=
VAPID_SUBJECT=mailto:ops@avtotest.uz
```

---

## 6. Follow-ups (not this slice)

- Retention digests / FSRS due reminders (product copy + cron)
- Arena invite push
- Admin broadcasts (M3)
- Align with M6 offline SW if caching is added later (merge carefully)
