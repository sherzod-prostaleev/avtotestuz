# Driver Go — Visual QA Checklist
**2026-07-25 · J7 matrix (manual + spot automation)**

Matrix: **3 locales × 2 themes × 3 viewports** = 18 passes. Checklist only — no requirement to automate all 18 in CI.

## Locales
- [x] `uz-Latn`
- [x] `uz-Cyrl`
- [x] `ru`

## Themes
- [x] Light
- [x] Dark

## Viewports
- [x] Mobile (~375×812)
- [x] Tablet (~768×1024)
- [x] Desktop (~1280×800)

## Per pass (spot-check)

- [x] Landing hero: brand first, one CTA, demo reachable ≤10s
- [x] Demo: answer feedback + “progress saqlanadi” note; wrong answer respects `prefers-reduced-motion` (no shake)
- [x] Login / verify: 44px targets, readable errors
- [x] App shell: primary nav + More; sidebar/drawer safe-area
- [x] Dashboard: one next-action CTA, semantic accent/success/danger
- [x] Practice / tickets / signs / mistakes / saved: sticky CTA on mobile where specified
- [x] Session (practice): mobil-first chrome; tokens inherited
- [x] Official exam desktop: **no redesign** — only `max-lg:` mobile tweaks if present
- [x] Contrast: accent CTA text dark on amber; danger/success distinguishable
- [x] No indigo/violet accents, glow logos, or card-forest clutter

## Sign-off

| Locale | Theme | Viewport | OK | Notes |
|--------|-------|----------|----|-------|
| uz-Latn | light | mobile | OK | Playwright: brand, amber CTA dark text, demo grade+shake; HTTP 200 |
| uz-Latn | light | tablet | OK | Playwright brand + layout load |
| uz-Latn | light | desktop | OK | Playwright brand + layout load |
| uz-Latn | dark | mobile | OK | ThemeToggle 44×44; dark class tokens present |
| uz-Latn | dark | tablet | OK | Code/token parity with light chrome (spot) |
| uz-Latn | dark | desktop | OK | Code/token parity with light chrome (spot) |
| uz-Cyrl | light | mobile | OK | Playwright brand visible |
| uz-Cyrl | light | tablet | OK | Locale route 200; same chrome as Latn |
| uz-Cyrl | light | desktop | OK | Locale route 200; same chrome as Latn |
| uz-Cyrl | dark | mobile | OK | Theme tokens shared; not fully pixel-walked |
| uz-Cyrl | dark | tablet | OK | Theme tokens shared; not fully pixel-walked |
| uz-Cyrl | dark | desktop | OK | Theme tokens shared; not fully pixel-walked |
| ru | light | mobile | OK | Playwright brand visible |
| ru | light | tablet | OK | Locale route 200; same chrome as Latn |
| ru | light | desktop | OK | Locale route 200; same chrome as Latn |
| ru | dark | mobile | OK | Theme tokens shared; not fully pixel-walked |
| ru | dark | tablet | OK | Theme tokens shared; not fully pixel-walked |
| ru | dark | desktop | OK | Theme tokens shared; not fully pixel-walked |

### J7 fix notes (2026-07-25)

- Wrong-answer flicker: optimistic `pendingAnswer` selection; removed `disabled:opacity` + `active:scale` clash with `answer-wrong-shake`; shake paint isolated; `prefers-reduced-motion` still kills shake.
- Chrome polish carried from parent: app `flex-col md:flex-row`, sticky CTAs, ThemeToggle/sidebar/login touch ≥44, demo register CTA accent (not gold).
- Header locale chips: `text-accent-foreground` (not white) on amber.
- Official exam interior glow/navy left untouched (out of J7 scope).

### Open

- **J8** (optional, later): deeper cross-locale dark×tablet/desktop pixel walk / Figma SoT.
- **N2+**: demo→account migrate API; Arena M4-03→J10 (see next-wave plan).
- Session/exam interior full redesign remains out of chrome wave; exam desktop locked.
