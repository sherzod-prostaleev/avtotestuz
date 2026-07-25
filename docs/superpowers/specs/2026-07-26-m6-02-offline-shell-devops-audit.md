# Driver Go — DevOps Audit (M6 offline shell slice · U-39)

**Date:** 2026-07-26  
**Auditor:** Cursor agent  
**Repo:** `/home/sher/Рабочий стол/avtotest` · branch `main`  
**Scope:** Smallest solid PWA offline improvement on existing `/sw.js` — shell precache + navigation fallback.  
**Constraint:** No full offline exam / question catalog sync; push handlers preserved.

---

## 0. Executive verdict

**Green for merge.** Offline users hitting a previously visited public/shell route (or cold-open with no cache) get a branded `offline.html` instead of a blank failure. Static assets use cache-first; navigations stay network-first. API/BFF never cached.

---

## 1. Change surface

- `frontend/public/sw.js` — install/activate/fetch + existing push
- `frontend/public/offline.html` — static offline shell page
- `frontend/src/lib/pwa-sw-shell.test.ts` — contract tests
- `frontend/src/components/pwa/register-sw.tsx` — comment only
- Inventory U-39 → **partial** (shell done; content sync still open)

---

## 2. Frontend quality

| Check | Result |
|-------|--------|
| vitest `pwa-sw-shell` + `pwa-manifest` | pass |
| vitest web-push card / lib | pass (SW still exposes push) |
| `tsc --noEmit` | pass |

---

## 3–8. Ops / safety

- No backend / DB / seed changes
- No VAPID / payment / LLM secrets
- Auth-gated app routes are **not** precached; only public shell path regex is runtime-cached after a successful online visit
- Cache names versioned (`dg-shell-v1`, `dg-runtime-v1`); activate deletes unknown caches

---

## 9. Findings by severity

| Sev | Finding | Action |
|-----|---------|--------|
| Info | Full offline content sync (bilets/signs) still open | U-39 remainder / M6-02 |
| Info | Install prompt UX still optional | U-38 polish |

---

## 12. E0 gate

Safe to commit + push.
