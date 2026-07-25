# Driver Go — Design System v2 · Usability First
**Asphalt & Signal · 2026-07-25**

> Brand: **Driver Go**. Direction: **A — Asphalt & Signal**, re-grounded under **Usability First**.
> This document supersedes the spike framing for chrome implementation priority.
> Session/exam interior remains Phase J9 (token inheritance only until then).

---

## Priority order (locked)

1. **Usability** — ≤2 taps to main action; first demo question ≤10s; touch ≥44×44; feedback ≤100ms.
2. **Hierarchy / IA** — one job per screen; 6 primary nav + More.
3. **Honest progress** — readiness, streak, FSRS due; no fake social proof.
4. **Visual system** — color as tool (amber/success/danger/gold), not decoration.

If usability and visual conflict → **usability wins** (example: dark CTA text on amber for WCAG).

---

## Tokens (sync with `frontend/src/app/globals.css`)

| Token | Light HEX (approx) | Dark HEX (approx) | Role |
|-------|--------------------|-------------------|------|
| background | `#F3F4F6` | `#0E1116` | Asphalt day/night |
| card | `#FFFFFF` | `#171A21` | Surfaces |
| foreground | `#121721` | `#F3F5F7` | Text AAA |
| muted-fg | `#606876` | `#929CAA` | Secondary ≥ AA |
| accent | `#F09A05` | `#FAA40F` | Primary CTA |
| accent-fg | `#121721` | `#121721` | CTA text (dark!) |
| success | `#1A9E60` | `#20B670` | Correct |
| danger | `#DA281B` | `#E64337` | Wrong / destructive |
| streak | flame only | flame only | Quota |
| gold | VIP only | VIP only | Quota |

**Banned:** indigo/violet accents, purple blobs, glow logos, multi-layer glass, rainbow mode cards.

---

## Mobile-first chrome rules

- `.page-shell` / `.page-shell-narrow` / `.page-shell-tight` — consistent padding + safe-area bottom.
- `.sticky-cta-bar` — primary CTA stays in thumb zone on phones; static on `sm+`.
- `.filter-chip` + `.chip-scroll` — horizontal snap filters (tickets, signs, leaderboard).
- `.field-input` / `.back-link` / `.touch-target` — min-height 44px.
- Mobile header: 44px menu + streak; drawer `min(18rem, 88vw)` with safe-area insets.
- Signs detail: bottom sheet on mobile, centered modal on `sm+`.
- Cards: flat border surfaces (no backdrop-blur glass tax on low-end phones).

---

## Chrome pages (J6) checklist

| Page | Mobile sticky CTA | Semantic colors | page-shell |
|------|-------------------|-----------------|------------|
| Dashboard | next-action full width | accent/success/danger modes | ✓ |
| Practice | start CTA | ✓ | ✓ |
| Tickets | next ticket CTA | progress = accent | ✓ |
| Signs | bottom-sheet modal | ✓ | ✓ |
| Mistakes | review/upgrade CTA | gold VIP gate | ✓ |
| Saved | browse CTA | no gradient | ✓ |
| Premium | full-width buy | gold popular | ✓ |
| Profile | save CTA | field-input 44px | ✓ |
| Stats | due CTA 44px | ✓ | ✓ |
| Leaderboard | period chips | no gradient | ✓ |
| Checkout | centered card | ✓ | existing |

---

## Out of scope (still)

- `session/[id]` question play UI
- Official exam interior (`official-avtotest-exam-view`)
- Arena UI

---

## Next

1. J7 visual QA matrix: 3 locales × 2 themes × 3 viewports.
2. J9 session interior on same tokens.
3. Demo answer migration to account (localStorage → server) for investment continuity.

*Status: SOURCE OF TRUTH for chrome + mobile UX after 2026-07-25 implementation pass.*
