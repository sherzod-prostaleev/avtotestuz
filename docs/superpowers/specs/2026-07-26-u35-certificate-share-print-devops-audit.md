# U-35 certificate share/print improve — devops audit

**Date:** 2026-07-26  
**Scope:** Web Share API + clipboard fallback; public page Print → Save as PDF (honest, no server PDF library invented).

## Gates
| Check | Result |
|-------|--------|
| vitest `certificate-share.test.ts` | pass |
| Honesty | UI states browser print for PDF; no fake credential PDF binary |

## Remains
Admin-issued credential PDF with QR seal — still open if product needs it.
