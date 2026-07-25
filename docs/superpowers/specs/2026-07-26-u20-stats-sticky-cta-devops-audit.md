# Driver Go — DevOps Audit (U-20 · Stats sticky due CTA + token dots)

**Date:** 2026-07-26  
**Auditor:** Cursor agent  
**Repo:** `/home/sher/Рабочий стол/avtotest` · branch `main`  
**Scope:** N4 chrome debt — Stats page-level due sticky CTA; provider picker dots → Asphalt tokens.  
**Constraint:** No exam redesign; no purple theme; no seed wipe; no payment keys.

---

## 0. Executive verdict

**Green.** Stats due action is page-sticky (thumb zone) and routes to FSRS `mode=review` like the dashboard, not the mistakes hub.

---

## 1. Change surface

- `frontend/src/app/[locale]/(app)/stats/page.tsx` — `.sticky-cta-bar` when `dueCount > 0`; review session URL
- `stats/page.test.tsx` — CTA href + hide-when-zero cases
- `provider-picker.tsx` — `bg-accent` / `bg-gold` dots (was cyan/blue utilities)
- Inventory U-20 note updated (still partial: a11y / `no-img-element`)

---

## 2. Quality gates

| Check | Result |
|-------|--------|
| `vitest …/stats/page.test.tsx` | pass (2) |
| `vitest …/provider-picker.test.tsx` | pass |
| `tsc --noEmit` | pass |
| Seed / migrations | untouched |

---

## 3–8. Ops / safety

- No backend / env / compose changes
- Review deep-link reuses existing session start (`mode=review`) — same path as dashboard next-action
- Sticky only renders when due queue is non-empty

---

## 9. Findings

| Sev | Finding | Action |
|-----|---------|--------|
| Info | Content `no-img-element` (S3/CDN media) still eslint-disabled | Accept unless static logos migrate |
| Info | Footer contact placeholders (U-17) need marketing inputs | Separate |
| Info | Official exam hex palette intentionally locked | Do not touch |

---

## 12. E0 gate

Safe to commit + push.
