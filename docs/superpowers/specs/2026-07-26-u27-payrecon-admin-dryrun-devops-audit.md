# U-27 payrecon admin dry-run — devops audit

**Date:** 2026-07-26  
**Scope:** `GET /admin/v1/payments/recon?hours=` (`payments.read`) wraps existing `recon.Run` dry-run + Admin UI. Still **no** live Payme/Click statement APIs.

## Gates
| Check | Result |
|-------|--------|
| `go test ./internal/admin/ -run Payments` | pass (includes recon dry-run) |
| Honesty | response `dry_run=true` + note |

## Remains
Persist findings queue; outbound merchant statement when U-03 keys exist.
