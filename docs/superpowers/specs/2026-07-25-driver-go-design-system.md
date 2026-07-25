# Driver Go — Master Design System
**Asphalt & Signal · 2026-07-25**

> Brand: **Driver Go** (UI). Product domain may remain avtotest; user-facing name is Driver Go only.
> Direction locked: **A — Asphalt & Signal**.
> Out of scope this wave: in-session question UI + official exam simulation interior (token inheritance only).
> Competitive refs (principles only, no clone): prepdrive.uz, PassPilot, Duolingo press, Brilliant focus, Linear calm, Apple HIG / MD3 theme.

---

## A. Executive Summary

Driver Go — O‘zbekiston nazariy haydovchilik imtihoniga tayyorgarlik: **asphalt atmosfera + signal-amber CTA** bilan “yo‘lga chiqish” metaforasi. Birinchi viewport bitta kompozitsiya: brand → va’da → bitta asosiy harakat → phone mock (tayyorlik). Ichki app **bitta “Bugungi eng yaxshi qadam”** atrofida aylanadi; streak va readiness Hooked tsiklini (trigger → action → reward → investment) ochiq saqlaydi. Light/dark bir xil hierarchy; indigo SaaS / glow / card-forest taqiqlangan. Natija: chalg‘itmaydigan, mashqni odatga aylantiradigan, conversion va retention birga ishlaydigan tizim.

**Nega eng yaxshi:** Edtech’da odat = progress ko‘rinishi + past kognitiv yuk + darhol mustahkamlash; lokal kontekst (yo‘l/signal) brandni global indigo’dan ajratadi.

---

## B. Core Design Philosophy

1. **One job per screen** — sahifa bir savolga javob beradi (“bugun nima qilaman?”). BJ Fogg: Ability ↑ → Behavior ↑.
2. **Signal, don’t decorate** — rang = ma’no (amber=harakat, green=to‘g‘ri, red=xato, gold=VIP). Ornament = shovqin.
3. **Brand-first composition** — nav olib tashlansa ham “Driver Go” seziladi (brand test).
4. **Progress made visceral** — readiness %, streak, weak topic: raqam + vizual, lekin hero’da stat dump yo‘q.
5. **Honest motivation** — yolg‘on “minglab foydalanuvchi” yo‘q; isbot = demo savol, rasmiy format, FSRS.
6. **Tactile primary, calm chrome** — CTA’da 3D press (Duolingo press); qolgan UI Linear/Raycast tinchligi.
7. **Habit over novelty** — har qaytishda yangi “circus” emas; bir xil loop, kuchaygan mastery.

**Nega:** Kahneman System 1 birinchi 3s da ishlaydi; System 2 faqat demo/FAQ da. Csikszentmihalyi flow = challenge ≈ skill (daily target + zaif mavzu).

---

## C. Color · Type · Spacing

### C.1 Color (HSL tokens — kodda `globals.css` bilan sync)

| Token | Light | Dark | Rol |
|-------|-------|------|-----|
| `--background` | `220 16% 96%` | `220 22% 7%` | Asphalt day / night |
| `--foreground` | `220 28% 10%` | `210 20% 96%` | Matn |
| `--card` | `0 0% 100%` | `220 18% 11%` | Surface |
| `--border` | `220 12% 88%` | `220 14% 18%` | Ajratkich |
| `--muted-foreground` | `220 10% 42%` | `215 12% 62%` | Ikkinchi matn (≥4.5:1) |
| `--accent` | `38 96% 48%` | `38 96% 52%` | **Signal amber CTA** |
| `--accent-foreground` | `220 28% 8%` | `220 28% 8%` | CTA matn (qorong‘u!) |
| `--accent-shadow` | `38 90% 36%` | `38 85% 34%` | 3D press |
| `--success` | `152 72% 36%` | `152 70% 42%` | Traffic green |
| `--danger` | `4 78% 48%` | `4 78% 56%` | Stop red |
| `--streak` | `18 92% 48%` | `18 92% 54%` | Flame |
| `--gold` | `43 94% 48%` | `43 94% 52%` | VIP only |
| `--ring` | = accent | = accent | Focus |

**Taqiqlangan:** indigo/violet accent, purple blob background, multi-glow logo, neon disco dark.

**Nega:** Amber = diqqat/harakat (signal); green/red = o‘rganish feedback; asphalt = haydovchilik konteksti; WCAG AA kontrast majburiy.

### C.2 Typography

| Rol | Font | Weight | Size scale |
|-----|------|--------|------------|
| Display | Baloo 2 (`--font-baloo`) | 700–800 | 32 / 40 / 48 / 56 / 64 |
| Body | Manrope (`--font-manrope`) | 400–700 | 12 / 14 / 16 / 18 |
| Mono (OTP, scores) | Manrope tabular | 700 | 14–24 |

Line-height: display 1.08–1.15; body 1.5–1.65. Tracking: display tight; labels `0.08–0.2em` uppercase sparingly.

**Nega:** Baloo = do‘stona lekin “o‘yincha emoji” emas; Manrope = Cyrillic + Latin; Inter/Roboto default stack yo‘q.

### C.3 Spacing & radius

**Space:** 4 · 8 · 12 · 16 · 24 · 32 · 48 · 64 · 96 (faqat shu qadamlar).  
**Radius:** `sm=8` · `md=12` · `lg=16` · `xl=24` (token `--radius ≈ 14px` oilasi).  
**Shadow:** `shadow-3d` (CTA), `shadow-sm` optional; glass multi-layer yo‘q.  
**Max content:** 1120–1200px.

---

## D. Homepage — wireframe + psychology

```
┌─────────────────────────────────────────────────────────┐
│ [Logo Driver Go]                    [Theme] [Kirish]    │  sticky, 64px
├─────────────────────────────────────────────────────────┤
│ ASPHALT FULL-BLEED HERO (bir kompozitsiya)              │
│ ┌──────────────────────┐  ┌────────────────────────┐   │
│ │ DRIVER GO            │  │   ┌──────────────┐     │   │
│ │ Headline (1)         │  │   │ Phone mock   │     │   │
│ │ Sub (1 gap)          │  │   │ readiness+   │     │   │
│ │ [Bepul boshlash]     │  │   │ next action  │     │   │
│ │ [Ro‘yxatsiz sinab]→# │  │   └──────────────┘     │   │
│ └──────────────────────┘  └────────────────────────┘   │
├─────────────────────────────────────────────────────────┤
│ METHOD (3 qator, cardsiz) — esga / oralik / izoh        │  trust → science
├─────────────────────────────────────────────────────────┤
│ DEMO #demo — bitta haqiqiy savol                        │  competence
├─────────────────────────────────────────────────────────┤
│ WHY — 4 benefit, icon+text, border-top not cards        │  differentiation
├─────────────────────────────────────────────────────────┤
│ HOW — 1 2 3 big numerals                                │  ability
├─────────────────────────────────────────────────────────┤
│ FAQ — accordion                                         │  objection kill
├─────────────────────────────────────────────────────────┤
│ BOTTOM CTA — bitta tugma                                │  convert
├─────────────────────────────────────────────────────────┤
│ Footer: brand · desc · signs link                       │
└─────────────────────────────────────────────────────────┘
```

| Bo‘lim | Maqsad | Psixologiya |
|--------|--------|-------------|
| Hero | 3s da qolish + CTA | System 1; brand test; Fogg trigger |
| Method | “Bu o‘yin emas, fan” | Trust; investment prelude |
| Demo | Competence now | Instant gratification; Hooked action |
| Why | Tanlash sababi | Differentiation vs prepdrive clutter |
| How | Ability ↑ | Fogg Ability |
| FAQ | Narx/til/ishonch | Objection handling |
| Bottom CTA | Ikkinchi convert | Endowed progress after scroll |

**Hero ichida YO‘Q:** year badge, 3 hold-card, 4 fake stats, floating stickers.

**Nega eng yaxshi:** PassPilot’s “one promise” + Duolingo’s clear CTA + Brilliant’s calm teaching — birlashtirilgan, lokal signal brand bilan.

---

## E. User Flows (asosiy 5)

### E1. First open → first win (anonymous)
Landing → `#demo` yoki CTA → 1 savol → success/fail feedback → “Ro‘yxatdan o‘tish”.  
**Reward:** competence. **Investment:** xato/izoh ko‘rish istagi.

### E2. OTP login
Landing/Kirish → phone → OTP → dashboard. Referral `?ref=` saqlanadi.  
**Nega:** friction minimal; SMS trust line.

### E3. Daily return (Habit)
Push/open → Dashboard “Bugungi eng yaxshi qadam” → Practice/Resume → 10 savol → streak++ / readiness↑ → close.  
**Hooked:** Trigger (streak risk) → Action (1 CTA) → Variable reward (mastery/streak) → Investment (weak topic history).

### E4. Paywall / VIP
Free limit yoki Grand Mock lock → Premium → provider → pending poll → success → entitlement.  
**Nega:** returnURL locale-aware; pending = anxiety reduction.

### E5. Exam readiness
Readiness ≥ threshold + volume floor → Grand Mock unlock → (exam UI out of scope) → certificate moment on chrome.  
**Nega:** false “100%” dan himoya (volume floor); reward = status.

---

## F. Component Library

| Component | Spec | Nega |
|-----------|------|------|
| **Button / Primary** | Amber fill, `border-b-4` press, `text-accent-foreground`, h 44–52 | Tactile commit |
| **Button / Outline** | Border card, hover border-accent | Secondary |
| **Button / Gold** | VIP only | Scarcity without spam |
| **Input** | 44px+, focus ring accent, +998 prefix pattern | Error less |
| **OTP boxes** | 6× square, auto-advance, mono | Familiar bank pattern |
| **Nav link** | Active = accent fill + press shadow; inactive muted | Wayfinding |
| **Nav More** | Collapse secondary 5 items | Miller’s 7±2 |
| **Stat tile** | Label / big number / caption; skeleton same footprint | No layout shift |
| **Next-action card** | Title + desc + full-width CTA | One job |
| **Progress bar** | Success fill on muted track | Readiness visceral |
| **Mastery bar** | Existing; chrome only this wave | Consistency |
| **Trial countdown** | null while loading; VIP only when known | Free-user stability |
| **Empty / Error** | Icon + 1 sentence + 1 CTA | Recovery |
| **Skeleton** | `bg-border/60` pulse | Trust during fetch |
| **Phone mock** | Marketing only; mirrors dashboard truth | Product proof |
| **FAQ details** | Native `<details>`, no heavy JS | A11y + perf |

**Card qoida:** default = no card. Card faqat interaksiya yoki ajratilgan action uchun.

---

## G. Micro-interactions & Motion

| Motion | Duration | Easing | Qayerda |
|--------|----------|--------|---------|
| Page enter | 280–350ms | ease-out | Landing sections, auth card |
| CTA press | 150ms | linear | `active:translate-y-1` + shadow collapse |
| Theme crossfade | 200ms | ease | `next-themes` |
| Streak flame | 2s loop | ease-in-out | Streak only |
| Skeleton pulse | CSS | — | Loading |
| More chevron | 200ms | — | Sidebar |

**Qoidalar:**
- `prefers-reduced-motion: reduce` → animation ~0 (allaqachon globals’da).
- Landingda shimmer/flame yo‘q.
- Overlay stickers / confetti hero’da yo‘q.
- Success feedback session ichida (keyingi to‘lqin); chrome’da subtle readiness tick yetarli.

**Nega:** Motion = hierarchy va “jonlilik”; shovqin retention’ni o‘ldiradi (Arc/Linear darsi).

---

## H. Responsive strategy

| Breakpoint | Layout |
|------------|--------|
| **<640** mobile | Single column; sticky top bar; drawer nav; hero stacked (copy → phone); CTA full width |
| **640–1023** tablet | Hero 2-col if space; content max 720–960 |
| **≥1024** desktop | Sidebar 256px fixed; main `md:ml-64`; hero 2-col; max 1200 |

**Touch:** ≥44×44. **Safe areas:** notch padding. **Thumb zone:** primary CTA pastga yaqin mobil auth’da.

**Nega:** Mobil-first — auditoriya telefon-heavy; desktop = nafas, dashboard wall emas.

---

## I. Conversion & Retention mechanisms

### Conversion (anon → paid path)
1. Hero CTA = primary convert.
2. Demo = micro-convert (competence).
3. Honest FAQ (bilet bepul, tillar, manba).
4. Bottom CTA after investment (scroll).
5. Premium: clear tariff, provider picker, locale returnURL, pending reassurance.

### Retention (Hooked loop)
| Stage | Driver Go implementation |
|-------|--------------------------|
| Trigger | Streak at risk; due questions; Telegram later |
| Action | Dashboard single next-action CTA |
| Variable reward | Readiness jump, streak flame, weak-topic clear, leaderboard |
| Investment | Mistake bank, saved Qs, mastery history, referral |

### Anti-patterns (taqiqlangan)
- Fake social proof.
- 5 parallel CTAs.
- Dark pattern “VIP” flash before load (fixed).
- Notification spam without value.

**Nega:** Amplitude/Mixpanel best practice = one North Star (e.g. D1 practice start) + activation (first 10 answers) + habit (3-day streak).

---

## J. Implementation Roadmap

| Phase | Deliverable | Done? |
|-------|-------------|-------|
| **J0 Spike** | Direction A lock, brand Driver Go, scope | ✅ |
| **J1 Tokens** | globals Asphalt & Signal | ✅ started |
| **J2 Landing** | One-composition hero + sections | ✅ started |
| **J3 Shell** | Sidebar primary+More | ✅ started |
| **J4 Auth** | Login/verify chrome | ✅ started |
| **J5 Dashboard chrome** | Hero cleanup, indigo purge | ✅ started |
| **J6 Remaining chrome** | Premium, profile, practice, tickets, signs, mistakes, saved, stats, leaderboard, checkout | ✅ |
| **J6b Mobile UX** | page-shell, sticky-cta, 44px targets, chip-scroll, safe-area | ✅ |
| **J7 Motion/a11y pass** | Semantic token purge, contrast, reduced-motion, visual QA matrix | ✅ |
| **J8 Figma source of truth** | Tokens + key screens (optional but recommended) | ⬜ later |
| **J9 Session/exam interior** | Practice session mobil-first; exam desktop locked, mobile `max-lg:` only | ✅ |
| **J10 Arena UI** | On this system after M4-03 plan | ⬜ later |

**Figma → Dev:**
1. Figma variables = CSS tokens (C.1).
2. Components = F table.
3. Dev: Tailwind semantic classes only (`bg-accent`, not raw amber).
4. Visual QA matrix: 3 locales × 2 themes × 3 viewports.
5. No redesign mid-Arena: lock tokens before M4-03 UI.

**Nega bu tartib:** Token-first → marketing → shell → chrome → (session later) = ikki marta chizishdan saqlaydi.

---

## Appendix — Research synthesis (short)

| Source | Olingan | Olinmagan |
|--------|---------|-----------|
| Duolingo | Press CTA, streak as habit | Mascot spam, purple, badge rain |
| Brilliant | Calm focus, one next lesson | Math-only aesthetic |
| Linear/Raycast | Density + breathing room | Dev-tool coldness |
| PassPilot | One promise, honesty FAQ | UK App Store wall |
| Hooked | Trigger–action–reward–investment | Manipulation dark patterns |
| Fogg | Make action tiny (10 Q) | — |
| Apple HIG / MD3 | Contrast, touch, theme | Material purple |
| prepdrive | Local exam framing | Clutter, clone |

---

*Hujjat status: SOURCE OF TRUTH for chrome redesign. Session/exam interior = Phase J9.*
