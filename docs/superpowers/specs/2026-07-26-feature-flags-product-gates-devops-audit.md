# Feature flags product gates — devops audit

**Date:** 2026-07-26  
**Scope:** Wire `feature_flag` into real product surfaces (no longer admin-only dead table).

## Gates wired
| Flag | Consumer |
|------|----------|
| `arena_enabled` | `MintTicket` → 403 `feature_disabled`; FE Arena page via `GET /flags` |
| `checkout_payme` / `checkout_click` | AND with provider kill-switch in `ListProviderStatuses` + `EnsureProviderEnabled` |
| `web_push_digest` | `RunFSRSDueDigest` live send blocked (`ErrFeatureDisabled`); dry-run still OK |
| (snapshot) `maintenance_mode` | Exposed on `GET /api/v1/flags` for future FE |

## Delivered
- `internal/flags` reader + public `GET /api/v1/flags`
- Proxy marks `/flags` public
- Admin Flags UI note updated

## Tests
| Check | Result |
|-------|--------|
| `go test ./internal/flags/` | pass |
| `go test ./internal/arena/ -run MintTicket` | pass |
| `go test ./internal/billing/ -run ProviderKillSwitch` | pass |
| `go test ./internal/push/ -run DigestRespects` | pass |

## Remains
Teacher/B2B dashboard stub; U-50 inventory refresh; maintenance_mode FE chrome optional.
