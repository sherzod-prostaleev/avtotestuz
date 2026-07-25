# U-22 Leaderboard rebuild cap docs — devops audit

**Date:** 2026-07-26  
**Scope:** Documentation + CLI usage note only; no scoring formula change.

## Secrets / hygiene
- No runtime behavior change beyond stderr usage hint.
- No secrets.

## Tests
| Check | Result |
|-------|--------|
| Existing `TestRebuildPeriodAppliesDailyCap` | unchanged / still authoritative |
| Docs link from CLI package comment | present |

## Remains
None for U-22. Reopen only if product makes leaderboard money-critical and needs historical VIP timelines.
