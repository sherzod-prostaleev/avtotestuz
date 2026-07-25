# U-29 Leftover bilets UX copy — devops audit

**Date:** 2026-07-26  
**Scope:** Honest i18n copy on Tickets hero + Practice variant-source hint. No content/schema change; no seed wipe.

## Secrets / hygiene
- Copy only; no PII; no invented question counts in UI (avoids drift if leftover count changes).

## Product honesty
- States leftovers exist from import pairing, work in practice/FSRS, absent from numbered bilet list.
- Does not invent a fake “bilet 62” or reassign questions.

## Tests
| Check | Result |
|-------|--------|
| vitest tickets page | pass (asserts leftover note) |

## Remains
Optional later: admin content UI listing unassigned ext_ids (M3). Not required for U-29 UX honesty.
