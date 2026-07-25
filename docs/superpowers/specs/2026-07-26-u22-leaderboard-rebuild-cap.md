# U-22 Leaderboard rebuild daily-cap approximation

**Status:** Accepted trade-off (documented) · 2026-07-26  
**Code:** `leaderboard.Service.RebuildPeriod` · CLI `cmd/rebuildleaderboard`  
**Design SoT:** `2026-07-25-m4-01-leaderboard-design.md` §5

## What rebuild does
Recomputes Redis sorted-set scores from durable `session_answer` (correct answers), applying the same **per-day** `leaderboard_daily_points` cap that live `RecordPoint` enforces, then summing capped day totals for the period.

## Approximation (not a bug)
VIP status and cap values are taken from **current** `entitlement` / `limit_config` at rebuild time. The schema does **not** store historical VIP/cap timelines. Therefore:

| Guarantee | Not guaranteed |
|-----------|----------------|
| Rebuild never uncapped-farming-explodes scores after Redis loss | Byte-identical scores vs the live board if VIP/cap changed mid-window |
| Free users stay under free cap; VIP under VIP cap (as of now) | Perfect historical fidelity for users who upgraded mid-period |

Worst case: small score drift after VIP/cap change during the rebuilt window; self-heals on subsequent live play and future rebuilds under the new status.

## Ops
```bash
# from backend/
go run ./cmd/rebuildleaderboard -period all
# or: daily | weekly | monthly | alltime
```

Safe to re-run anytime. Prefer after Redis flush/restart or if live vs Postgres drift is suspected.

## Why not “fix” historical fidelity
Would require durable VIP/cap history (or event sourcing) for every profile-day — cost/complexity outweighs a low-risk ranking approximation. Revisit only if leaderboard becomes money-critical.
