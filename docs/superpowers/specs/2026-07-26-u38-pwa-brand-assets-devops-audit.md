# U-38 PWA icons / brand assets — devops audit

**Date:** 2026-07-26  
**Scope:** Asphalt mark SVG refresh, PNG install icons, apple-touch, SW/manifest/offline shell alignment, BrandLogo rounded mark.

## Secrets / hygiene
- No VAPID, payment, or LLM keys touched.
- Excluded from commit: `.worktrees/`, `node_modules/`.

## Tests
| Check | Result |
|-------|--------|
| `vitest` `pwa-manifest.test.ts` + `brand-logo.test.tsx` | pass |

## Notes
- Manifest uses `/logo-512.png` (maskable) + `/icon.svg`.
- Layout apple icon → `/apple-touch-icon.png` (180×180).
- Design exploration PNGs under `docs/logo-variants/` (not runtime).
