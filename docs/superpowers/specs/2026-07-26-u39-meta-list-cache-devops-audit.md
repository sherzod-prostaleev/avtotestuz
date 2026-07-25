# U-39 offline metadata list cache — devops audit

**Date:** 2026-07-26  
**Scope:** Expand SW network-first cache from bilets variants to also `categories` + `signs` list GETs via `/api/proxy/*`. Still **no** question bodies / exam sync.

## Gates
| Check | Result |
|-------|--------|
| vitest `pwa-sw-shell.test.ts` | pass |
| Honesty | comment + tests assert no IndexedDB question catalog |

## Remains
Full offline exam/question sync still open.
