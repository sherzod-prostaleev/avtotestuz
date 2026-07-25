# U-11 FSRS push digest `-send` — devops audit

**Date:** 2026-07-26  
**Scope:** Real `cmd/pushdigest -send` for FSRS/AI-tahlil due retention digests (no LLM).

## Secrets / hygiene
- No VAPID keys invented or committed; `-send` fails cleanly when unconfigured.
- No payment/LLM secrets touched.

## Tests
| Check | Result |
|-------|--------|
| `go test ./internal/push/...` | pass |
| `go build ./cmd/pushdigest` | pass |

## Behavior
- Dry-run (default): candidate count only.
- `-send`: batch Notify `kind=fsrs_due`, locale copy + `/…/session/start?mode=review`, prune 410/404 subs.
- 20h per-profile cooldown via `notification` rows.
