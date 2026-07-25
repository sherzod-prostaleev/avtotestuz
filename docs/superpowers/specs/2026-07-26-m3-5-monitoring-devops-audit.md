# M3-5 Monitoring — devops audit

**Date:** 2026-07-26  
**Scope:** Admin monitoring slice — `/admin/v1/monitoring/health` (Postgres/Redis checks) + `/metrics` (U-41 process counters via injected snapshot) + `/jobs` (honest CLI catalog) + `/{locale}/admin/monitoring/{health,perf,jobs}`. Public `/healthz|/readyz|/metrics` unchanged. Ops health marked deprecated with Admin link.

## Verdict
**Green** for M3-5 practical slice. No Prometheus/Sentry/SSE invented. No fake running workers or pause/resume. No staging host.

## Gates
| Check | Result |
|-------|--------|
| `go test ./internal/admin/ -run Monitoring` | pass (health/metrics/jobs + finance deny / analyst allow) |
| Permission | `monitoring.read` |
| BFF | `/api/admin/monitoring/{health,metrics,jobs}` |
| Honesty | jobs `status=manual` `kind=cli`; metrics process-local |
| Safety | no seed wipe; public probes remain for k8s |

## Remains
Live logs tail, alert rules, host CPU/disk, multi-instance metrics — still open (U-41 depth / M3 later). Next: M3-6 Analytics tiles.
