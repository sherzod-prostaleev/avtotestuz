# Ops → Admin deprecation harden — devops audit

**Date:** 2026-07-26  
**Scope:** Mark remaining `/{locale}/ops/*` surfaces as deprecated bridges with links to Admin SoT equivalents. Ops token APIs kept as escape hatch (not deleted).

## Mapping
| Ops | Admin home |
|-----|------------|
| `/ops/health` | `/admin/monitoring/health` |
| `/ops/contacts` | `/admin/cms/chrome` |
| `/ops/providers` | `/admin/payments/providers` |
| `/ops/users` | `/admin/users` |
| `/ops/payments` | `/admin/payments/transactions` |
| `/ops/audit` | `/admin/security/audit` (still stub) |
| `/ops/limits` | `/admin/settings/config` (still stub) |

## Gates
| Check | Result |
|-------|--------|
| vitest ops-deprecated-banner | pass |
| Safety | no ops API removal; no seed wipe |

## Remains
Delete ops routes after operators fully on Admin; implement admin audit + limits write UI.
