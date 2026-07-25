# Driver Go — Visual QA Checklist
**2026-07-25 · J7 matrix (manual)**

Matrix: **3 locales × 2 themes × 3 viewports** = 18 passes. Checklist only — no requirement to automate all 18 in CI.

## Locales
- [ ] `uz-Latn`
- [ ] `uz-Cyrl`
- [ ] `ru`

## Themes
- [ ] Light
- [ ] Dark

## Viewports
- [ ] Mobile (~375×812)
- [ ] Tablet (~768×1024)
- [ ] Desktop (~1280×800)

## Per pass (spot-check)

- [ ] Landing hero: brand first, one CTA, demo reachable ≤10s
- [ ] Demo: answer feedback + “progress saqlanadi” note; wrong answer respects `prefers-reduced-motion` (no shake)
- [ ] Login / verify: 44px targets, readable errors
- [ ] App shell: primary nav + More; sidebar/drawer safe-area
- [ ] Dashboard: one next-action CTA, semantic accent/success/danger
- [ ] Practice / tickets / signs / mistakes / saved: sticky CTA on mobile where specified
- [ ] Session (practice): mobil-first chrome; tokens inherited
- [ ] Official exam desktop: **no redesign** — only `max-lg:` mobile tweaks if present
- [ ] Contrast: accent CTA text dark on amber; danger/success distinguishable
- [ ] No indigo/violet accents, glow logos, or card-forest clutter

## Sign-off

| Locale | Theme | Viewport | OK | Notes |
|--------|-------|----------|----|-------|
| uz-Latn | light | mobile | | |
| uz-Latn | light | tablet | | |
| uz-Latn | light | desktop | | |
| uz-Latn | dark | mobile | | |
| uz-Latn | dark | tablet | | |
| uz-Latn | dark | desktop | | |
| uz-Cyrl | light | mobile | | |
| uz-Cyrl | light | tablet | | |
| uz-Cyrl | light | desktop | | |
| uz-Cyrl | dark | mobile | | |
| uz-Cyrl | dark | tablet | | |
| uz-Cyrl | dark | desktop | | |
| ru | light | mobile | | |
| ru | light | tablet | | |
| ru | light | desktop | | |
| ru | dark | mobile | | |
| ru | dark | tablet | | |
| ru | dark | desktop | | |
