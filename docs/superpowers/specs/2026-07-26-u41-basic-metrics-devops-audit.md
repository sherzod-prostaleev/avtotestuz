# U-41 Basic process metrics — devops audit

**Date:** 2026-07-26  
**Scope:** In-process request counters + `GET /metrics` JSON; FE ops health aggregates it. No Prometheus/Grafana/Sentry.

## Secrets / hygiene
- `/metrics` is public (same class as `/healthz`) — counters only, no PII, no route labels that could leak IDs.
- Does not scrape auth headers or bodies.
- Probe paths `/healthz|/readyz|/metrics` excluded from counters.

## Safety
| Check | Result |
|-------|--------|
| ResponseWriter Hijacker/Flusher | implemented (Arena WS must keep working) |
| Process-local only | yes — resets on restart |
| Full Prometheus stack | intentionally not invented |

## Tests
| Check | Result |
|-------|--------|
| `go test ./internal/server/` | pass |
| vitest `api/ops/health/route.test.ts` | pass |

## Ops
- Staging: `curl -sS $API/metrics` → envelope with `requests_total`.
- FE: `/{locale}/ops/health` shows uptime + totals.

## Remains
Tracing, alerting, Prometheus exposition, multi-instance aggregation — still open under U-41.
