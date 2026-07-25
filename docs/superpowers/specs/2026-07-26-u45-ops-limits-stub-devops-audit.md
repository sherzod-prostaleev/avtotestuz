# U-45 M3 ops limits stub — devops audit

**Date:** 2026-07-26  
**Scope:** `GET /ops/limits` + FE `/{locale}/ops/limits` read-only `limit_config` directory.

## Secrets / hygiene
- Ops token required (`X-Ops-Token`).
- Read-only — no PATCH/edit of gates (full Admin later).
- No PII in response.

## Tests
| Check | Result |
|-------|--------|
| `go test ./internal/ops/` | pass |
| vitest ops-nav | pass |

## Remains
U-45 still partial: RBAC, CMS, refunds UI, investor dashboards, write path for limits.
