# U-23 Drop dead `referral_attribution` — devops audit

**Date:** 2026-07-26  
**Scope:** Migration `0028` drops unused `referral_attribution` (created in `0003`). Live referral path remains `referral` / `user_referral_code` (`0015`).

## Secrets / hygiene
- No data migration of rows (table unused by app; empty in normal deploys).
- No change to payment/referral grant paths.
- Down migration recreates empty 0003-shaped table only (rollback hygiene).

## Safety
| Check | Result |
|-------|--------|
| App Go references to `referral_attribution` / `ReferralAttribution` | none (sqlc model removed after generate) |
| Live `ApplyReferralCode` / reward path | still on `referral` |
| Seed wipe | no |
| Truncate list | dropped dead table name |

## Tests
| Check | Result |
|-------|--------|
| `go test ./internal/db/...` | pass (asserts table absent) |
| `go test ./internal/billing/` | pass |
| `make generate` | idempotent; model gone |

## Deploy note
Apply migrate through `0028` on staging/prod with normal migrate path. No app downtime required beyond migrate lock.

## Rollback
`migrate down 1` restores empty `referral_attribution`; app still does not use it.
