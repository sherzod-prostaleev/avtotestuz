# M3-6 Analytics overview — devops audit

**Date:** 2026-07-26  
**Scope:** Honest product/revenue tiles — `GET /admin/v1/analytics/overview` (`analytics.read`) + `/{locale}/admin/analytics/overview`. SQL aggregates from `profile`, `payment` (paid), `entitlement` (active window), `event` (7d top names). Explicitly **not** MRR/churn/LTV BI.

## Verdict
**Green** for M3-6 basic tiles. No fake funnels, no ClickHouse, no invented KPIs.

## Gates
| Check | Result |
|-------|--------|
| `go test ./internal/admin/ -run Analytics` | pass |
| Permission | `analytics.read` (finance/analyst/admin/superadmin) |
| BFF | `/api/admin/analytics/overview` |
| Honesty | note field + UI copy; counts from live tables only |

## Remains
Funnels, exports, investor dashboards (U-46), cohort LTV. Next: M3-7 feature flags or support broadcast stub.
