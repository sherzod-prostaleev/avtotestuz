# U-27 Payment recon skeleton — devops audit

**Date:** 2026-07-26  
**Scope:** Dry-run local payment↔provider txn consistency (`cmd/payrecon`).

## Secrets / hygiene
- No Payme/Click merchant keys required or invented.
- Does not call external provider APIs.

## Tests
| Check | Result |
|-------|--------|
| `go test ./internal/billing/recon/...` | pass |
| `go build ./cmd/payrecon` | pass |

## Follow-up
- Persist findings to admin queue (M3).
- Optional outbound statement fetch when prod keys exist.
