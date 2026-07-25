# U-41 Prometheus text `/metrics` — devops audit

**Date:** 2026-07-26  
**Scope:** Upgrade public `GET /metrics` to Prometheus text exposition; keep JSON for FE; document `SENTRY_DSN` stub (no SDK).

## Secrets / hygiene
- `/metrics` still public — counters only, no route/PII labels.
- `SENTRY_DSN` loaded from env, unused by runtime until an SDK is wired.
- Probes `/healthz|/readyz|/metrics` excluded from counters.

## Compatibility
| Client | Behavior |
|--------|----------|
| Default / Prometheus scrape | `text/plain; version=0.0.4` series `avtotest_*` |
| `Accept: application/json` or `?format=json` | existing httpx envelope (ops health + admin) |
| Admin `GET /admin/v1/monitoring/metrics` | unchanged Snapshot map |

## Safety
| Check | Result |
|-------|--------|
| Hijacker/Flusher | preserved |
| Full Grafana/Sentry stack | not invented |
| FE ops health | still requests JSON |

## Tests
| Check | Result |
|-------|--------|
| `go test ./internal/server/` | pass (JSON + Prometheus + negotiation) |
| `go test ./internal/config/` | pass |

## Remains
Sentry SDK wiring, tracing, multi-instance aggregation, alerting pager — still open.
