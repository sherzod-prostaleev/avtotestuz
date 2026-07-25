# Monitoring ops feed + alert_rule thin slice — devops audit

**Date:** 2026-07-26  
**Scope:** Admin ops feed (admin_audit_log + payment fails) + `alert_rule` stub (2 static rules) evaluated on health + alerts page. Not zap/SSE/Sentry.

## Delivered
| Surface | Detail |
|---------|--------|
| Migration | `0035_alert_rule` (postgres_ready, payment_fails_24h) |
| Feed API | `GET /admin/v1/monitoring/feed` |
| Alerts API | `GET /admin/v1/monitoring/alerts` |
| Health | Includes live `alerts` evaluation |
| UI | `/{locale}/admin/monitoring/{logs,alerts}` (sidebar unstubbed) |
| Permission | `monitoring.read` |

## Verdict
**Green** thin slice. Honest: no log shipper, no pager, no inventing host metrics.

## Gates
| Check | Result |
|-------|--------|
| `go test ./internal/admin/ -run Monitoring` | pass |
| `go test ./internal/db/ -run MigrateCreates` | pass |
| Safety | alert_rule seeded, not wiped by Truncate (like limit_config) |

## Remains
U-39 done-enough note; U-50 refresh; Prometheus/Sentry still open.
