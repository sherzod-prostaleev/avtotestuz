# Sahifalar orasidagi o'tish animatsiyasi — dizayn

Sana: 2026-08-30
Holat: tasdiqlangan, implementatsiyaga tayyor

## 1. Nima uchun

Foydalanuvchi `app.osonprava.uz` da sahifalar orasidagi o'tish "silliq" ekanini
aytdi va bizda ham shunday, imkon bo'lsa yaxshiroq qilishni so'radi.

**osonprava tahlili** (bundle `index-BEF9cdUw.js` va CSS o'qildi):

- Framer Motion ishlatadi — `motion.div` 68 marta, `AnimatePresence` 22 marta.
- Sahifa o'tishi naqshi:
  `<AnimatePresence mode="wait">` ichida
  `initial={{opacity:0, x:30}} animate={{opacity:1, x:0}} exit={{opacity:0, x:-30}}`,
  `transition={{duration:.25}}` — ya'ni **so'nish + 30px gorizontal siljish, 250ms**.
- Bundle'da React Router'ning `startViewTransition` kodi bor, lekin CSS'da bironta
  `::view-transition` qoidasi YO'Q — demak View Transitions API amalda ishlatilmaydi.

**Bizning holat:** animatsiya kutubxonasi umuman yo'q. Faqat CSS `transition`
utilitalari va qo'lda yozilgan `Reveal` (scroll-reveal, IntersectionObserver).

**Foydalanuvchi tanlovi:** to'rt variantdan **"yumshoq so'nish"** — siljishsiz,
faqat shaffoflik, ~180–200ms. Sabab: eng tinch, eng arzon, eski sinf
kompyuterlarida eng bir tekis.

## 2. Nega View Transitions, Framer Motion emas

| | View Transitions (tanlandi) | Framer Motion | Faqat CSS `template.tsx` |
|---|---|---|---|
| Qo'shiladigan JS | ~0 KB | ~35 KB gz | 0 KB |
| Chiqish animatsiyasi | bor (brauzer eski DOM'ni suratga oladi) | App Router'da hiyla kerak (FrozenRouter) | **yo'q** |
| Eski PC'da og'irligi | GPU kompozit, eng yengil | JS har kadrda | yengil |
| Sidebar'ni qimirlatmaslik | `view-transition-name` bilan aniq boshqariladi | qo'lda | qo'lda |
| Qo'llamaydigan brauzer | animatsiyasiz o'tadi, buzilmaydi | ishlaydi | ishlaydi |

Tanlangan "yumshoq so'nish" — aynan brauzerning o'zi eng yaxshi bajaradigan ish.
Framer Motion'ning kuchi bu holatda ortiqcha, narxi esa real (kiosk B2B sinf
PC'larida ishlaydi, `chrome.exe --kiosk`, zaxira `msedge.exe`).

**osonprava'dan yaxshiroq bo'ladigan joy:** bizda doimiy sidebar va header bor.
Faqat kontent maydoniga `view-transition-name` beriladi, sidebar/header
nomlanmaydi — natijada ular mutlaqo qimirlamaydi, faqat o'rta qism eriydi.
osonprava'da butun ekran siljiydi.

## 3. React 19 migratsiyasi

Next'ning `experimental.viewTransition` bayrog'i React 19 talab qiladi
(`react@18.3.1` da `unstable_ViewTransition` yo'q; `next@16.2.12`
`config-schema.js:315` da bayroq bor).

**O'lchandi** (`spike/react-19` shoxchasida haqiqiy migratsiya bajarildi):

- Olib tashlangan React 19 API'lari (`findDOMNode`, `defaultProps`, string ref,
  `ReactDOM.render`, `unmountComponentAtNode`) — kodda **bittasi ham yo'q**.
- Peer mosligi: Radix (dialog/slot/tooltip) `^19` qo'llaydi; TanStack
  query/virtual/table, zustand, lucide — ochiq peer; `@testing-library/react`
  16.3.2 `^19` qo'llaydi. **Yagona to'siq** — `next-themes@0.3.0` (peer
  `^16.8 || ^17 || ^18`), `0.4.6` da React 19 bor.
- `1.0.0-beta.0` mavjud, lekin beta — olinmaydi. Chegara: `^0.4.6`.

Natija: `npm install` peer konfliktisiz, 0 zaiflik; `tsc` toza; `eslint` toza;
`next build` muvaffaqiyatli; **688 unit** va **56 e2e** test o'tdi; mavzu
almashishi brauzerda jonli tekshirildi (dark↔light, `localStorage`,
`colorScheme`, aria-label — hammasi to'g'ri).

Yagona kod tuzatishi: `use-fit-scale.test.ts` dagi 2 ta `@ts-expect-error`
keraksiz bo'lib qoldi (React 19'da `RefObject.current` tip bo'yicha
o'zgaruvchan) — olib tashlanadi.

Paketlar:

```
next-themes      ^0.3.0  → ^0.4.6
react            ^18.3.0 → ^19.2.8
react-dom        ^18.3.0 → ^19.2.8
@types/react     ^18.3.0 → ^19.2.18
@types/react-dom ^18.3.0 → ^19.2.5
```

## 4. Animatsiya arxitekturasi

**Yoqish.** `next.config.ts` → `experimental: { viewTransition: true }`.

**Nomlash.** Faqat kontent maydoni nomlanadi. `[locale]/layout.tsx` dagi umumiy
shell emas, balki har bir route-guruhning kontent o'ramiga bitta barqaror nom:

- `(app)` — `AppShell` ichidagi `<main>`
- `(public)`, `(kiosk)`, `(auth)`, `(session)` — mos kontent elementi

Sidebar, header va pastki navigatsiya **nomlanmaydi** — nomlanmagan element
snapshot'ga tushmaydi va animatsiyalanmaydi, ya'ni joyida qotib turadi.

**CSS** (`globals.css`), taxminan 15 satr:

```css
::view-transition-old(content),
::view-transition-new(content) {
  animation-duration: 180ms;
  animation-timing-function: ease;
}
```

Standart cross-fade yetarli — `mix-blend-mode` yoki qo'shimcha keyframe kerak emas.

**Degradatsiya, ikki qatlam:**

1. `@media (prefers-reduced-motion: reduce)` → `animation: none`. Bu shart:
   `globals.css:140` da loyihada allaqachon shu tamoyil bor.
2. `@supports not (view-transition-name: none)` → eski kiosk brauzeri uchun
   `template.tsx` orqali oddiy 180ms `fade-in` (faqat kirish). View Transitions
   qo'llanadigan joyda bu qoida umuman qo'llanmaydi, ya'ni ikkitasi ustma-ust
   tushmaydi.

## 5. Test rejasi

- **Unit:** yangi mantiq deyarli yo'q (CSS + bitta konfig bayrog'i), shuning
  uchun yangi unit test yozilmaydi. Mavjud 688 test regressiyani ushlaydi.
- **E2E:** `prefers-reduced-motion` bilan navigatsiya hali ham ishlashini
  tekshiruvchi bitta spec. Animatsiyaning o'zini piksel bo'yicha tekshirish
  mo'rt bo'ladi — buni qilmaymiz.
- **Qo'lda:** brauzerda `dashboard ↔ biletlar ↔ mashq` o'tishini ko'z bilan
  ko'rish; sidebar qimirlamasligini tasdiqlash.

## 6. Chiqarish tartibi

Ikkita alohida commit, ikkalasi ham to'liq gate'dan (unit + e2e + build + lint)
o'tgandan keyin:

1. `chore(deps): React 19` — faqat paketlar va `@ts-expect-error` tozalash.
   Bu commit'dan keyin sayt aynan hozirgidek ishlaydi, animatsiya yo'q.
2. `feat(ui): sahifa o'tishlarida cross-fade` — konfig, nomlash, CSS, zaxira.

Ikki bosqichga bo'lish sababi: agar React 19'da kutilmagan regressiya chiqsa,
uni animatsiyadan ajratib qaytarish mumkin bo'ladi.

## 7. Qamrovdan tashqarida

- Yo'nalishli (oldinga/orqaga) animatsiya — foydalanuvchi "yumshoq so'nish"ni
  tanladi, siljish yo'q.
- Shared-element (bilet kartasi → bilet sahifasi morph) — keyingi bosqichga.
- Framer Motion kiritish — kerak emas.
