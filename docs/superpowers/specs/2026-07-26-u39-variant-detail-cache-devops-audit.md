# U-39 offline — recently opened ticket (variant) detail cache

**Date:** 2026-07-26  
**Scope:** Cache grading-neutral `GET /api/proxy/variants/{n}` payloads for tickets
the learner actually opens. Prefetch on ticket start so the SW sees the request.

## Done this slice
| Change | Detail |
|--------|--------|
| SW `VARIANT_DETAIL_RE` | network-first; `dg-variant-v1`; max 20 entries |
| Prefetch | `prefetchVariantDetail` on tickets start |
| BFF public | `variants` added to proxy public paths (matches BE) |

## Explicit remaining gap (large — do not pretend closed)
- Offline **exam session** create / answer submit / scoring
- Question **images** / media package
- Full catalog sync / background sync / conflict resolution
- Authenticated progress / FSRS deck offline

Usable claim: **re-read recently opened ticket question text offline** if the
variant was fetched while online. **Not** a full offline exam.

## Gates
| Check | Result |
|-------|--------|
| vitest `pwa-sw-shell` / `prefetch-variant` / tickets / proxy | run in commit gate |

Inventory **U-39 stays partial**.
