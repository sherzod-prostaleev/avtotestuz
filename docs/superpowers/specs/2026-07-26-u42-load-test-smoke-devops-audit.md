# U-42 Load-test smoke (k6) — devops audit

**Date:** 2026-07-26  
**Scope:** k6 smoke script + `make load-test` + optional `workflow_dispatch` CI. Not a prod soak.

## Secrets / hygiene
- No hostnames invented; `API_BASE` is operator-supplied.
- Script hits only public probes + public content list GETs (no auth tokens).
- CI workflow is dispatch-only — not a required gate; no secrets required.

## Safety
| Check | Result |
|-------|--------|
| Seed / DB wipe | not touched |
| Payment / Arena WS | not hit |
| Fake staging host | none |

## Artifacts
| Path | Role |
|------|------|
| `deploy/load-test/smoke.js` | k6 script |
| `deploy/load-test/README.md` | operator notes |
| `Makefile` `load-test` | local entrypoint |
| `.github/workflows/load-test.yml` | optional dispatch |

## Honesty
Thresholds are “smoke not on fire” (`http_req_failed<5%`, p95&lt;1.5s). Capacity planning, soak, and authenticated journeys remain open under U-42.

## Remains
Full perf audit (N+1, index review), soak against real staging, auth/checkout load profiles — still open; need host (U-02) for meaningful remote runs.
