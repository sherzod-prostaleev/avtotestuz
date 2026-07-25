# U-35 Grand Mock certificate persistence — devops audit

**Date:** 2026-07-26  
**Scope:** Persist shareable certificate id on Grand Mock pass (vertical slice).

## Secrets / hygiene
- No PII beyond score/total/issued_at on public endpoint.
- No payment/LLM keys.

## Tests
| Check | Result |
|-------|--------|
| `go test ./internal/session/... ./internal/db/...` | pass |
| `vitest use-session-engine.test.ts` | pass |

## Ship surface
- Migration `0027_grand_mock_certificate`
- Finish/idempotent + public lookup APIs
- FE dialog share link + public `/sertifikat/[code]` page
