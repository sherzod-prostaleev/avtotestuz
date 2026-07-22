# AvtoTest Frontend

Next.js 14 (App Router) + TypeScript + Tailwind CSS. Phase A status: design
system + 3 static mockup pages (no real API calls yet — see
`docs/superpowers/specs/2026-07-22-nextjs-frontend-foundation-design.md`).

## Dev

```bash
npm install
npm run dev        # http://localhost:3000
npm run lint
npm run typecheck
npm test
npm run build
```

## Phase A mockup routes

- `/` — Landing (hero, interactive static demo question, proof stats, features, FAQ)
- `/dashboard` — Dashboard (streak, readiness ring, 4 nav cards)
- `/exam-mockup` — Exam screen, with a state switcher demonstrating unanswered /
  correct / incorrect / exam-hidden (anti-cheat, no feedback) visual states

None of these call the real Go backend yet — all data comes from
`src/lib/mock-data.ts`. Phase B wires real auth, TanStack Query/Zustand, and
the remaining pages.
