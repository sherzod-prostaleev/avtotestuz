# Maintenance mode FE chrome — devops audit

**Date:** 2026-07-26  
**Scope:** Surface `feature_flag.maintenance_mode` to learners via `GET /flags` + app-shell banner.

## Delivered
- `MaintenanceBanner` in `(app)/layout` (above support banner)
- vitest for on/off
- Toggle remains Admin → Settings → Feature flags (`settings.flags`)

## Verdict
**Green.** Honest strip only — does not hard-block routes (no invented kill-switch UX beyond the flag).

## Gates
| Check | Result |
|-------|--------|
| vitest `maintenance-banner.test.tsx` | pass |
| Safety | no seed wipe; uses existing public `/flags` |

## Remains
Admin limits write; support inbox; homepage CMS.
