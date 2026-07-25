# U-39 Offline bilets list cache — devops audit

**Date:** 2026-07-26  
**Scope:** SW network-first cache for variants bilets list API only.

## Secrets / hygiene
- No auth tokens stored in SW beyond normal request cache of authenticated GET responses the browser already made.
- Does not cache question bodies / exam payloads.

## Tests
| Check | Result |
|-------|--------|
| vitest `pwa-sw-shell.test.ts` | pass |
