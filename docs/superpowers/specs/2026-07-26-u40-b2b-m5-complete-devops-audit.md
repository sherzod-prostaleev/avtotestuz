# U-40 B2B M5 complete (invite/write/stats) — devops audit

**Date:** 2026-07-26  
**Scope:** Finish M5 code-completable slice — mig `0036` `b2b_invite`, teacher write actions, org stats + CSV, admin completeness (invite/role/remove/stats/export).

## Delivered
| Surface | Detail |
|---------|--------|
| BE teacher | invite/list/accept, remove member, change role, licenses, stats, `export.csv` |
| BE admin | stats, CSV, invite-by-phone, PATCH role, DELETE member; detail includes `seats_used` |
| FE | `/{locale}/teacher` write UI; profile invite accept card; admin org detail write + CSV |
| Proxy | learner + admin proxies pass through `text/csv` |

## Gates
| Check | Result |
|-------|--------|
| `go test ./internal/b2b/ -run Teacher` | pass |
| `go test ./internal/admin/ -run B2B` | pass |
| Safety | no seed wipe; last-owner protected; grant still Admin |

## Remains
School **sales / self-serve seat billing** when a paying customer appears. Not invented here.
