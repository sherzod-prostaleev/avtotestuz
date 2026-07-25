# U-45 M3 ops users directory stub — devops audit

**Date:** 2026-07-26  
**Scope:** `GET /ops/users` + FE `/ops/users` (masked phone).

## Secrets / hygiene
- Ops token required; phones masked in API response.
- No payment/LLM keys.

## Tests
| Check | Result |
|-------|--------|
| `go test ./internal/ops/...` | pass |
| vitest `ops-nav.test.tsx` | pass |
