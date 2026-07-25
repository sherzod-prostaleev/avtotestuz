# U-45 M3 ops payments + audit stubs — devops audit

**Date:** 2026-07-26  
**Scope:** `GET /ops/payments`, `GET /ops/audit` + FE pages.

## Secrets / hygiene
- Ops token required; no secrets in responses.
- Audit is read-only list (no before/after dump in stub).

## Tests
| Check | Result |
|-------|--------|
| `go test ./internal/ops/...` | pass |
| vitest ops-nav | pass |
