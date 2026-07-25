# U-39 offline — public site CMS cache slice + remaining gap

**Date:** 2026-07-26  
**Scope:** One more thin offline slice: network-first cache for public
`/api/proxy/site/{contacts,banner,home}`. Reaffirm **done-enough** ceiling.

## Done this slice
| Change | Detail |
|--------|--------|
| SW `META_LIST_RE` | + `site/contacts\|banner\|home` |
| `META_CACHE` | bump `dg-meta-v1` → `dg-meta-v2` |
| Proxy public allow | `site/home` treated like contacts/banner |

## Explicit remaining gap (large — do not pretend closed)
- Offline **exam session** / question bodies / answer keys
- Question **images** catalog + media offline package
- Background sync / conflict resolution / stale content invalidation
- Authenticated progress / FSRS deck offline

Inventory **U-39 stays partial** until a dedicated offline-exam plan ships.

## Gates
| Check | Result |
|-------|--------|
| vitest `pwa-sw-shell.test.ts` | run in commit gate |
