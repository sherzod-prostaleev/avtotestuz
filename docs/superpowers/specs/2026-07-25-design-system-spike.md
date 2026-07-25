# DriveGo / AvtoTest — Design System Spike (2026-07-25)

> **Maqsad:** landing + marketing + app chrome (shell) ni **super** darajaga chiqarish.
> Prepdrive.uz — ilhom / raqobat, **clone emas**. Undan yaxshiroq bo‘lish shart.
>
> **Scope ICHIGA kiradi:** public landing, login/verify, dashboard shell, sidebar/nav, premium, profile, practice hub, tickets list, signs catalogue, mistakes/saved/stats/leaderboard chrome, checkout pending/success/failure, referral/public pages, theme (light+dark), typography, tokens, motion, empty/loading/error micro-states.
>
> **Scope DAN TASHQARI (o‘zgartirilmaydi bu to‘lqinda):**
> - `session/[id]` ichidagi savol o‘ynash UI
> - rasmiy imtihon simulyatsiyasi (`official-avtotest-exam-view`, exam-mockup ichki sahna)
> - `QuestionCard` / `AnswerOption` / `QuestionStage` / timer ichki loop (keyinroq alohida, agar kerak bo‘lsa)

---

## 0. Sherzodning fikri ×10 (qulflangan brief)

### 0.1 Mahsulot hissi
Foydalanuvchi birinchi 3 soniyada tushunishi kerak: **bu imtihonni o‘tish uchun mashq qiladigan joy**, o‘yincha dashboard emas. Keyin: “bugun 10 daqiqa qilsam, ertaga yanada tayyorroq bo‘laman.”

### 0.2 “Undan yaxshiroq” = nima demak
Prepdrive (va o‘xshashlar) ko‘pincha: ko‘p CTA, ko‘p badge, ko‘p feature grid, zaif brand. Bizda aksincha:

| Prepdrive tipik | DriveGo super |
|-----------------|---------------|
| Ko‘p bo‘lim, chalg‘ituvchi | **Bir kompozitsiya** — brand + 1 headline + 1 gap + 1 CTA guruh + 1 dominant visual |
| Generic SaaS indigo | **O‘ziga xos palitra** — yo‘l / signal / O‘zbekiston konteksti |
| Stat strip hero ichida | Hero da **faqat** brand + promise + CTA + visual |
| Card forest | Card faqat **interaksiya** uchun |
| Dark-by-default glow | Light + dark **ikkala** ideal; glow spam yo‘q |
| “Boshlash” × 5 | Bitta asosiy harakat: **mashqni boshlash** |

### 0.3 Qurilmalar
- **Phone (320–430):** barmoq zonasi, sticky CTA, bitta vertikal oqim, sidebar drawer.
- **Tablet (768–1024):** 2 ustun faqat kerak joyda; hero hali full-bleed.
- **Desktop (1280+):** max-width ~1120–1200; bo‘sh joy nafas oladi, “dashboard wall” emas.
- Touch target ≥ 44×44; focus ring har doim; reduced-motion hurmati.

### 0.4 Light + dark
Ikkala rejim bir xil **hierarxiya** va kontrast (≥ WCAG AA). Dark = tungi mashq uchun tinch navy/charcoal, neon disco emas. Light = issiq qog‘oz/asphalt oq emas, lekin “cream serif terracotta” cliché ham emas.

### 0.5 Psixologiya (marketing + learning science)
- **Immediate competence:** landingda 1 ta demo savol (mavjud) — lekin u hero ni siqmasin; alohida “sinab ko‘r” bo‘lim.
- **Progress made visible:** readiness / streak / weak topic — lekin landingda raqamlar hero emas, isbot emas (hozirgi 1235/61/13/3 — shubhali “proof”).
- **One next action:** har sahifada bitta aniq keyingi qadam.
- **Trust without fake social proof:** “minglab foydalanuvchi” degan yolg‘onni olib tashlash; o‘rniga aniq mahsulot isboti (savol banki, til, rasmiy format).
- **Retention loop (app chrome):** “Bugungi eng yaxshi qadam” — dashboard da bitta recommendation (allaqachon bor) — vizual jihatdan shu markaziy.

### 0.6 Mikro-detallar (majburiy checklist)
- Brand hero-level: nav olib tashlanganda ham sahifa “DriveGo” ekanligi seziladi.
- Logo glow / gradient text — kamaytiriladi yoki olib tashlanadi (hozirgi emerald glow + indigo gradient = AI default).
- Icon row / pill cluster / floating badge overlay — taqiqlangan (user design rules).
- Spacing scale: 4/8/12/16/24/32/48/64 — ixtiyoriy `p-3.5` chaos yo‘q.
- Radius: bitta oila (masalan 12/16/24), `rounded-full` pill spam yo‘q.
- Shadow: max 1–2 daraja; multi-layer glass yo‘q.
- Motion: 2–3 intentional (page enter, CTA press, theme crossfade) — flamePulse/shimmer faqat streak/gold uchun, landingda emas.
- Empty / loading / error: bir xil skeleton tili (VIP flicker darsi).
- i18n: uz-Latn / uz-Cyrl / ru — matn uzunligi uchun layout “break” qilmasin.
- Image: real yo‘l belgisi / mahsulot UI mock — abstract blob hero emas.

---

## 1. Hozirgi holat — nega “juda yomon”

Audit (`globals.css` + landing + sidebar):

1. **AI default palitra:** accent = indigo/violet (`238 84%`), body da purple/gold radial blob, feature iconlar violet/amber/emerald/sky — brand yo‘q, template bor.
2. **User rule buzilishi:** “AVOID purple-on-white / purple-to-indigo” — aynan shu.
3. **Hero clutter:** badge + title + subtitle + 2 CTA + 3 hold card + 4 stat — birinchi viewport “dashboard”.
4. **Brand zaif:** “DriveGo” gradient text + glow logo; nav olib tashlansa boshqa edtech bo‘lishi mumkin.
5. **Card forest:** glass-card everywhere; olib tashlasa ham tushunarli bo‘lgan joylar ko‘p.
6. **Sidebar:** 11+ nav item — cognitive overload; mobil header glow logo + flame.
7. **Fake-ish proof:** “Minglab…” copy + arbitrary stats.
8. **3D Duolingo tugma** + “premium glass” aralashmasi — ikki til birga.

Xulosa: muammo “rangni almashtirish” emas — **kompozitsiya + brand + hierarchy + token reset**.

---

## 2. Reference (clone qilinmaydi — prinsip olinadi)

| Reference | Olinadigan prinsip | OlinMAYDI |
|-----------|-------------------|-----------|
| prepdrive.uz | Lokal bozor, imtihonga tayyorlash framing | Layout clone, rang, section tartibi |
| PassPilot | Bitta promise, offline/trust, readiness language, FAQ rostgo‘ylik | UK-only copy, App Store wall |
| James May Theory | “Small bit, often” + spaced repetition story | Celebrity, cluttered feature dump |
| Duolingo (faqat press) | Tactile CTA press, daily loop | Green mascot spam, purple, endless badges |
| Linear / Raycast marketing | Typography hierarchy, calm density | Dev-tool aesthetic |

---

## 3. Uchta yo‘nalish (bitta tanlanadi)

### A — **Asphalt & Signal** (tavsiya)
- **Mood:** tungi shahar yo‘li + signal chiroq: charcoal asphalt, signal amber CTA, traffic green success, stop red danger.
- **Visual:** full-bleed road/sign photography yoki crisp product phone mock (readiness + “bugungi mashq”).
- **Type:** kuchli display (hozirgi Baloo saqlash mumkin yoki yanada “signage” grotesk) + Manrope body.
- **Nega super:** O‘zbekiston haydovchilik imtihoniga eng yaqin metafora; indigo SaaS dan uzoq; light/dark tabiiy.
- **Xavf:** juda “automotive brochure” bo‘lib ketishi — CTA va learning warmth bilan muvozanat.

### B — **Daylight Studio**
- **Mood:** kunduzgi o‘quv studiyasi: issiq off-white, deep teal accent (indigo emas), soft charcoal type.
- **Visual:** belgi katalogi / kitob stoli atmosfera, lekin real product UI.
- **Nega super:** tinch fokus, uzoq mashq uchun ideal; dark mode = tungi studio.
- **Xavf:** “cream + serif + terracotta” cliché ga yaqinlashmaslik kerak — teal + grotesk majburiy.

### C — **Kinetic Coach**
- **Mood:** energiya + coach: charcoal + vivid road-green, 3D press tugmalar saqlanadi, lekin glass/glow olib tashlanadi.
- **Visual:** bitta katta phone mock + “keyingi 10 savol” loop.
- **Nega super:** retention / streak / arena uchun keyinroq yaxshi mos.
- **Xavf:** yana gamification shovqiniga tushish; qattiq intizom kerak.

**Spike tavsiyasi:** **A (Asphalt & Signal)** — brand testidan o‘tadi, raqobatchidan uzoq, imtihon mahsulotiga mos.

---

## 4. Token skeleton (tanlovdan keyin to‘ldiriladi)

```
--bg, --fg, --muted
--surface, --surface-2, --border
--brand (CTA), --brand-press
--success, --danger, --warning (signal)
--streak, --vip/gold (kam ishlatiladi)
--radius-sm|md|lg
--space-1…8
--font-display, --font-body
--motion-fast|med (respect prefers-reduced-motion)
```

Indigo/violet accent **o‘chiriladi**. Emerald logo glow default bo‘lmaydi.

---

## 5. Implementatsiya bosqichlari (tanlovdan keyin)

1. **T0 — Tokens + typography + ThemeToggle** (globals + tailwind) — session/exam ichiga tegmaydi, lekin ranglar meros bo‘ladi (ichki savol UI ham yangi accent oladi — bu OK, layout o‘zgarmaydi).
2. **T1 — Public landing** (bitta kompozitsiya hero, keyin sectionlar qayta yoziladi).
3. **T2 — Auth (login/verify)**.
4. **T3 — App shell** (sidebar/nav hierarchy qisqartirish + mobile).
5. **T4 — Chrome pages** (dashboard frame, premium, profile, lists) — **session/exam ichi skip**.
6. **T5 — Micro-states** (empty/loading/error) + motion polish + a11y pass.
7. **T6 — Visual QA** phone/tablet/desktop × light/dark.

Har bosqich: vitest + screenshot checklist; Arena implementatsiyasi shu tokenlar ustida ketadi.

---

## 6. Qulflangan qarorlar (2026-07-25, Sherzod + eng zo‘r defaultlar)

| # | Savol | Qaror | Nega |
|---|--------|--------|------|
| 1 | Yo‘nalish | **A — Asphalt & Signal** | Imtihon metaforasi, indigo SaaS dan uzoq, light/dark tabiiy |
| 2 | Brand UI | Faqat **Driver Go** (AvtoTest UI da yo‘q) | Bitta brand signal; repo/domen alohida qolishi mumkin |
| 3 | Hero visual | **Full-bleed asphalt/atmosfera + bitta phone mock** | Atmosfera = brand test; phone = mahsulot isboti. Ketma-ket 2 hero emas — **bir kompozitsiya** |
| 4 | Sidebar | **Ha — 5–6 primary + “Ko‘proq”** | 11+ item cognitive overload; mashq loop markazda |

### Hero tafsiloti (majburiy)
- Edge-to-edge visual plane (yo‘l/asphalt gradient + subtle signage texture yoki foto) — inset card hero emas.
- Ustida: **Driver Go** (hero-level) → 1 headline → 1 gap → 1 CTA guruh.
- O‘ng/past (desktop): bitta phone mock — “Bugungi mashq” / readiness; overlay badge/sticker yo‘q.
- Stat strip, hold-card uchligi, year pill — hero dan chiqariladi.

### Sidebar primary (qulflangan ro‘yxat)
1. Bosh sahifa (dashboard)
2. Bugungi mashq / Practice
3. Biletlar
4. Imtihon
5. Belgilar
6. Premium (VIP)

**Ko‘proq:** Xatolar, Saqlangan, Reyting, Statistika, Profil.

---

**Keyingi:** T0 tokens (`globals.css`) → T1 landing → T2 auth → T3 shell.
