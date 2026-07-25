# Driver Go — DevOps Audit (M4-08 Web Push foundation · U-11)

**Date:** 2026-07-26  
**Auditor:** Cursor agent  
**Repo:** `/home/sher/Рабочий стол/avtotest` · branch `main`  
**Scope:** Web push subscribe/send foundation (BE + profile FE + push-only SW).  
**Sources of truth:** `2026-07-26-m4-08-web-push-design.md`, inventory U-11.

---

## 0. Executive verdict

**Ship-ready foundation** for local/staging once operators set `VAPID_PUBLIC_KEY` / `VAPID_PRIVATE_KEY`. Without keys, API correctly reports `configured: false` and rejects subscribe with `web_push_unconfigured`. No seed wipe, no exam redesign, no purple chrome changes.

---

## 1. Git / change surface

### 1.1 Status
Uncommitted until this stage commit: migration `0026`, `internal/push`, FE profile card, `public/sw.js`, i18n, design + inventory note.

### 1.2 Committed surface (intended)
- BE: schema + sqlc + handlers + VAPID sender (`webpush-go`)
- FE: `WebPushCard`, `lib/web-push`, messages ×3
- Docs: design + this audit + inventory U-11 → partial

### 1.3 Risk areas
- Service worker at `/sw.js` is push-only; future M6 offline SW must merge carefully
- Real delivery needs valid VAPID + HTTPS origin (localhost may be limited by browser)

---

## 2. Frontend quality

| Check | Result |
|-------|--------|
| vitest `web-push` + `web-push-card` | pass |
| No purple redesign / hero clutter | N/A (profile card only) |

---

## 3. Backend quality

| Check | Result |
|-------|--------|
| `go test ./internal/push/...` | pass |
| `go test ./internal/config/...` | pass |
| Partial VAPID (one key only) | rejected at `Config.validate` |

---

## 4. Security / secrets

- VAPID keys env-only; not committed
- Subscribe requires JWT
- Endpoint must be `https://`
- Gone/404 from push service → subscription deleted

---

## 5. Design-system compliance

Profile card mirrors `TelegramLinkCard` (Asphalt accent icon, game/outline buttons). No new color theme.

---

## 6. Usability invariants

- Unconfigured state is explicit (same UX pattern as Telegram bot)
- Unsupported browsers get a clear message
- Test-send only after local enable

---

## 7. Runtime ops

```
VAPID_PUBLIC_KEY=
VAPID_PRIVATE_KEY=
VAPID_SUBJECT=mailto:ops@avtotest.uz
```

Generate: `npx web-push generate-vapid-keys`

---

## 8. Data safety

- Additive migration only (`push_subscription` + channel check widen)
- No content seed/wipe
- Truncate list updated for tests

---

## 9. Findings by severity

| Sev | Finding | Action |
|-----|---------|--------|
| Info | Campaigns / cron / admin broadcast deferred | Follow-up |
| Info | Full PWA offline still U-38/U-39 | Separate |

---

## 10. Remaining big work backlog

See inventory: U-02 remote host, U-03 payment keys, U-10 TG quiz, U-12 real LLM, U-38 PWA, U-45 M3 admin.

---

## 11. Audit agent actions

Implemented foundation; ran package tests above; committing this stage.

---

## 12. E0 gate close-out

Foundation green for merge to `main` after commit + push. Production delivery still blocked on operator VAPID keys (and HTTPS host from D18).
