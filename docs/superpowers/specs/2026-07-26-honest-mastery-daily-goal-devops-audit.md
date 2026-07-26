# 2026-07-26 — Honest mastery / daily goal fix — devops audit

**Focus:** Stats «Tayyorlik» + category bars were accuracy-on-attempts (inflated); daily soft goal was 10.

## Change
- Mastery/readiness = `(studied/total) × (correct/seen)` over full valid question bank
- API categories now expose `studied` + `total`
- Soft daily goal `daily_goal_default` → free 30 / vip 40; migration `0037` + streak backfill
- FE bars show `N/Total`; streak copy «Bugun: X savol · maqsad Y»

## Gates
| Gate | Result |
|------|--------|
| `go test ./internal/learning ./internal/progress ./internal/b2b` | PASS |
| `golangci-lint` (touched pkgs) | PASS |
| `go build ./cmd/api` | PASS |
| `tsc` + vitest (stats hooks / mastery-bar / i18n) | PASS |

## Remaining
FSRS due queue still only covers previously graded questions (by design). 100% readiness requires covering ~all valid questions with good accuracy — not inventing Metabase.
