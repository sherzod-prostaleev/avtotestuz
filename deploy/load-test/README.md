# U-42 — smoke load-test (k6)

Light **smoke** against a running API. This is **not** a production soak, capacity
study, or authenticated journey suite.

## Prerequisites

- API up (`./run.sh`, `make run`, or compose overlay) with Postgres+Redis healthy
- [k6](https://k6.io/docs/get-started/installation/) on `PATH`

## Run

```bash
# From repo root (defaults: API_BASE=http://localhost:8090, 5 VUs, 30s)
make load-test

# Override
API_BASE=http://localhost:8080 VUS=3 DURATION=15s make load-test
k6 run -e API_BASE=http://127.0.0.1:8090 deploy/load-test/smoke.js
```

## What it hits

| Path | Why |
|------|-----|
| `GET /healthz` | Liveness |
| `GET /readyz` | Readiness (DB+Redis) |
| `GET /metrics` | Process counters (Prometheus text preferred) |
| `GET /api/v1/{categories,variants,signs}` | Public content lists |

## What it does **not** do

- Auth / OTP / session exams
- Payme/Click webhooks or checkout
- Arena WebSocket
- Multi-hour soak or ramp-to-fail
- Fake remote hosts — point `API_BASE` at something you actually run

## CI

Optional GitHub Actions workflow `load-test.yml` is **workflow_dispatch** only
(no required PR check). It expects you to supply a reachable `API_BASE` input;
default local CI runners have no staging API, so the job is operator-triggered.
