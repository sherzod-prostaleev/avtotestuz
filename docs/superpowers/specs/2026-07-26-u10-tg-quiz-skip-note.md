# U-10 TG daily quiz — inventory skip note

**Date:** 2026-07-26  
**Decision:** **Skip** until user explicitly scopes M4-07.

## Why
M4-06 design §1.2 defers daily quiz delivery, scheduling, streak reminders, rich keyboards, multi-locale bot copy, flood limits, and `/unlink` to M4-07. There is **no tiny vertical** that ships a complete quiz without inventing scheduler + product UX.

## Not done
- Outbound quiz cron
- Quiz answer callbacks
- Invented notification copy beyond foundation bot

## Next
U-35 certificate share/print, then U-39 / U-27 / U-50.
