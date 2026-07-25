# M3-7 Feature flags — devops audit

**Date:** 2026-07-26  
**Scope:** `feature_flag` table (mig `0032`) + `GET/PATCH /admin/v1/settings/flags` (`settings.flags`) + `/{locale}/admin/settings/flags` boolean toggles + audit. Seeded keys: maintenance_mode, arena_enabled, web_push_digest, checkout_payme, checkout_click.

## Verdict
**Green** for M3-7 flags slice (picked over broadcast/push campaign — highest value small complete). Flags are **stored/audited**; product consumers still opt-in later (honest UI note). No invented percentage rollout engine UI beyond type validation.

## Gates
| Check | Result |
|-------|--------|
| `go test ./internal/admin/ -run FeatureFlags` | pass |
| Migration | `0032_feature_flags` |
| Audit | `settings.flags.patch` |
| Safety | no seed wipe of content corpus; type-validated values |

## Remains
Wire flags into FE/BE gates; support broadcast stub; push campaign admin. Next: ops→admin harden, then U-40 B2B.
