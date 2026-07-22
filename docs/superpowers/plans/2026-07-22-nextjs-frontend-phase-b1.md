# Next.js Frontend Phase B1 — Auth + BFF + i18n Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Real phone+OTP login backed by httpOnly-cookie sessions and a single-flight-refresh BFF proxy, plus locale-routed i18n (uz-Latn/uz-Cyrl/ru) covering the 3 existing Phase A pages. Dashboard/exam-mockup still show mock data after login — real data wiring is Phase B2.

**Architecture:** Next.js Route Handlers act as the only code that ever holds a Bearer token; all client code talks to same-origin `/api/proxy/*` and `/api/auth/*`. `next-intl` locale-routes every page under `[locale]/`. No client auth-state library — middleware + server-side cookie reads are the only gate in this phase.

**Tech Stack:** next-intl ^3.19, existing Next.js 14/TanStack-uninstalled-stack from Phase A.

## Global Constraints

- Repo path `/home/sher/Рабочий стол/avtotest` — Cyrillic+space, always double-quote.
- Locales: `uz-Latn` (default), `uz-Cyrl`, `ru` — exact strings, matching the backend's existing `?locale=` convention.
- Cookies: `at` (access token) and `rt` (refresh token), both `httpOnly`, `sameSite: "lax"`, `secure: process.env.NODE_ENV === "production"`. `at` maxAge 900s (15min), `rt` maxAge 2592000s (30 days).
- Backend base URL: `process.env.BACKEND_URL`, default `"http://localhost:8090"` (matches this repo's documented dev port in the root README).
- No Zustand in this plan (approved spec decision — first real use is Phase B3).
- Content strings from `lib/mock-data.ts` are NOT translated (demo/mockup content, not real i18n scope per the approved B1 spec) — only UI-chrome text (headings, labels, buttons, nav card titles/descriptions) moves into `messages/*.json`. The exam-mockup's "MUHIM" explanation paragraph is content-like and stays hardoded (documented exception, matches the mock-data carve-out spirit).
- Every task's acceptance: `npm run lint && npm run typecheck && npm test && npm run build` clean.

---

### Task 1: next-intl setup + locale-route the 3 existing pages

**Files:**
- Create: `frontend/src/i18n/config.ts`
- Create: `frontend/src/i18n/request.ts`
- Create: `frontend/messages/uz-Latn.json`, `frontend/messages/uz-Cyrl.json`, `frontend/messages/ru.json`
- Create: `frontend/src/middleware.ts` (NOTE: must live under `src/` since this project uses a `src/` directory — Next.js silently never invokes a `middleware.ts` at the wrong location, no error)
- Modify: `frontend/next.config.mjs`
- Delete: `frontend/src/app/layout.tsx`
- Create: `frontend/src/app/[locale]/layout.tsx` (replaces it, adds locale+intl)
- Move+modify: `frontend/src/app/(public)/page.tsx` → `frontend/src/app/[locale]/(public)/page.tsx`
- Move+modify: `frontend/src/app/(public)/demo-question-block.tsx` → `frontend/src/app/[locale]/(public)/demo-question-block.tsx`
- Move+modify: `frontend/src/app/(public)/page.test.tsx` → `frontend/src/app/[locale]/(public)/page.test.tsx`
- Move+modify: `frontend/src/app/(app)/dashboard/page.tsx` + `.test.tsx` → `frontend/src/app/[locale]/(app)/dashboard/`
- Move+modify: `frontend/src/app/(app)/exam-mockup/page.tsx` + `.test.tsx` → `frontend/src/app/[locale]/(app)/exam-mockup/`

**Interfaces:**
- Produces: `locales: readonly ["uz-Latn","uz-Cyrl","ru"]`, `defaultLocale: "uz-Latn"` (from `@/i18n/config`) — every later task's middleware/layout code imports these. Message key namespaces: `Landing.*`, `Dashboard.*`, `ExamMockup.*`, `ThemeToggle.*`.

- [ ] **Step 1: Install next-intl**

```bash
cd "/home/sher/Рабочий стол/avtotest/frontend" && npm install next-intl@^3.19.0
```

- [ ] **Step 2: Create the locale config and next-intl request config**

`frontend/src/i18n/config.ts`:

```ts
export const locales = ["uz-Latn", "uz-Cyrl", "ru"] as const;
export type Locale = (typeof locales)[number];
export const defaultLocale: Locale = "uz-Latn";
```

`frontend/src/i18n/request.ts`:

```ts
import { getRequestConfig } from "next-intl/server";
import { notFound } from "next/navigation";
import { locales } from "./config";

export default getRequestConfig(async ({ locale }) => {
  if (!locales.includes(locale as (typeof locales)[number])) notFound();
  return {
    messages: (await import(`../../messages/${locale}.json`)).default,
  };
});
```

- [ ] **Step 3: Wrap next.config.mjs with the next-intl plugin**

`frontend/next.config.mjs`:

```js
import createNextIntlPlugin from "next-intl/plugin";

const withNextIntl = createNextIntlPlugin("./src/i18n/request.ts");

/** @type {import('next').NextConfig} */
const nextConfig = {};

export default withNextIntl(nextConfig);
```

- [ ] **Step 4: Write the message files**

`frontend/messages/uz-Latn.json`:

```json
{
  "ThemeToggle": {
    "toLight": "Yorug' temaga o'tish",
    "toDark": "Qorong'i temaga o'tish"
  },
  "Landing": {
    "heroTitleBefore": "Prava'ni",
    "heroTitleAccent": "oson",
    "heroTitleAfter": "oling!",
    "heroSubtitle": "FSRS asosidagi aqlli o'quv dvigateli bilan haydovchilik nazariy imtihoniga tayyorlaning.",
    "ctaStart": "Bepul boshlash",
    "ctaNoSignup": "Ro'yxatsiz sinab ko'ring",
    "demoSectionTitle": "Hozir sinab ko'ring",
    "whyUsTitle": "Nega biz?",
    "feature1Title": "FSRS o'quv dvigateli",
    "feature1Text": "Har savol siz uchun aqlli takrorlash jadvali bilan due bo'ladi.",
    "feature2Title": "Tayyorlik %",
    "feature2Text": "Imtihonga qanchalik tayyorligingizni real vaqtda ko'ring.",
    "feature3Title": "YHQ-havolali izohlar",
    "feature3Text": "Har javobda huquqiy modda-havola bilan chuqur tushuntirish.",
    "feature4Title": "Real imtihon simulyatsiyasi",
    "feature4Text": "20 savol, 25 daqiqa, 3-xatoda to'xtash — aynan rasmiy qoida.",
    "howItWorksTitle": "Qanday ishlaydi",
    "step1Title": "Ro'yxatdan o'ting",
    "step1Text": "Telefon raqamingiz bilan bir daqiqada.",
    "step2Title": "Bilet yeching",
    "step2Text": "Birinchi bilet har doim bepul.",
    "step3Title": "Tayyorlikni kuzating",
    "step3Text": "Statistikada zaif mavzularni ko'ring.",
    "faqTitle": "Savol-javob",
    "faq1Q": "Birinchi bilet chindan bepulmi?",
    "faq1A": "Ha, ro'yxatdan o'tgan har bir foydalanuvchi uchun 1-bilet doim bepul.",
    "faq2Q": "Savollar qayerdan olingan?",
    "faq2A": "Real imtihon savollari, litsenziyasi tasdiqlangan manbadan.",
    "faq3Q": "Necha tilda ishlaydi?",
    "faq3A": "uz-Latn, uz-Cyrl va rus tillarida.",
    "footer": "AvtoTest — {year}"
  },
  "Dashboard": {
    "welcome": "Xush kelibsiz,",
    "planPremium": "Premium",
    "planFree": "Bepul reja",
    "streakSuffix": "kunlik streak",
    "streakToday": "Bugun: {done}/{goal} savol",
    "readinessLabel": "Tayyorlik",
    "navVariantsTitle": "Biletlar",
    "navVariantsDesc": "61 ta bilet, ketma-ket ochiladi",
    "navExamTitle": "Imtihon simulyatsiyasi",
    "navExamDesc": "20 savol, 25 daqiqa",
    "navPracticeTitle": "Mashq",
    "navPracticeDesc": "Mavzu yoki belgi bo'yicha",
    "navMistakesTitle": "Xatolar ustida ishlash",
    "navMistakesDesc": "FSRS asosida takrorlash"
  },
  "ExamMockup": {
    "modeUnanswered": "Javobsiz",
    "modeCorrect": "To'g'ri javob berilgan",
    "modeIncorrect": "Xato javob berilgan",
    "modeExamHidden": "Imtihon rejimi (feedback yashirin)",
    "positionLabel": "{n} / {total}"
  }
}
```

`frontend/messages/uz-Cyrl.json`:

```json
{
  "ThemeToggle": {
    "toLight": "Ёруғ темага ўтиш",
    "toDark": "Қоронғи темага ўтиш"
  },
  "Landing": {
    "heroTitleBefore": "Праванi",
    "heroTitleAccent": "осон",
    "heroTitleAfter": "олинг!",
    "heroSubtitle": "FSRS асосидаги ақлли ўқув двигатели билан ҳайдовчилик назарий имтиҳонига тайёрланинг.",
    "ctaStart": "Бепул бошлаш",
    "ctaNoSignup": "Рўйхатсиз синаб кўринг",
    "demoSectionTitle": "Ҳозир синаб кўринг",
    "whyUsTitle": "Нега биз?",
    "feature1Title": "FSRS ўқув двигатели",
    "feature1Text": "Ҳар савол сиз учун ақлли такрорлаш жадвали билан due бўлади.",
    "feature2Title": "Тайёрлик %",
    "feature2Text": "Имтиҳонга қанчалик тайёрлигингизни реал вақтда кўринг.",
    "feature3Title": "ЙҲҚ-ҳаволали изоҳлар",
    "feature3Text": "Ҳар жавобда ҳуқуқий модда-ҳавола билан чуқур тушунтириш.",
    "feature4Title": "Реал имтиҳон симуляцияси",
    "feature4Text": "20 савол, 25 дақиқа, 3-хатода тўхташ — айнан расмий қоида.",
    "howItWorksTitle": "Қандай ишлайди",
    "step1Title": "Рўйхатдан ўтинг",
    "step1Text": "Телефон рақамингиз билан бир дақиқада.",
    "step2Title": "Билет ечинг",
    "step2Text": "Биринчи билет ҳар доим бепул.",
    "step3Title": "Тайёрликни кузатинг",
    "step3Text": "Статистикада заиф мавзуларни кўринг.",
    "faqTitle": "Савол-жавоб",
    "faq1Q": "Биринчи билет чиндан бепулми?",
    "faq1A": "Ҳа, рўйхатдан ўтган ҳар бир фойдаланувчи учун 1-билет доим бепул.",
    "faq2Q": "Саволлар қаердан олинган?",
    "faq2A": "Реал имтиҳон саволлари, лицензияси тасдиқланган манбадан.",
    "faq3Q": "Неча тилда ишлайди?",
    "faq3A": "уз-Латн, уз-Кирилл ва рус тилларида.",
    "footer": "АвтоТест — {year}"
  },
  "Dashboard": {
    "welcome": "Хуш келибсиз,",
    "planPremium": "Премиум",
    "planFree": "Бепул режа",
    "streakSuffix": "кунлик стрик",
    "streakToday": "Бугун: {done}/{goal} савол",
    "readinessLabel": "Тайёрлик",
    "navVariantsTitle": "Билетлар",
    "navVariantsDesc": "61 та билет, кетма-кет очилади",
    "navExamTitle": "Имтиҳон симуляцияси",
    "navExamDesc": "20 савол, 25 дақиқа",
    "navPracticeTitle": "Машқ",
    "navPracticeDesc": "Мавзу ёки белги бўйича",
    "navMistakesTitle": "Хатолар устида ишлаш",
    "navMistakesDesc": "FSRS асосида такрорлаш"
  },
  "ExamMockup": {
    "modeUnanswered": "Жавобсиз",
    "modeCorrect": "Тўғри жавоб берилган",
    "modeIncorrect": "Хато жавоб берилган",
    "modeExamHidden": "Имтиҳон режими (feedback яширин)",
    "positionLabel": "{n} / {total}"
  }
}
```

`frontend/messages/ru.json`:

```json
{
  "ThemeToggle": {
    "toLight": "Перейти к светлой теме",
    "toDark": "Перейти к тёмной теме"
  },
  "Landing": {
    "heroTitleBefore": "Получите",
    "heroTitleAccent": "права",
    "heroTitleAfter": "легко!",
    "heroSubtitle": "Готовьтесь к теоретическому экзамену с умным движком обучения на основе FSRS.",
    "ctaStart": "Начать бесплатно",
    "ctaNoSignup": "Попробуйте без регистрации",
    "demoSectionTitle": "Попробуйте прямо сейчас",
    "whyUsTitle": "Почему мы?",
    "feature1Title": "Движок обучения FSRS",
    "feature1Text": "Каждый вопрос становится due по умному расписанию повторения.",
    "feature2Title": "Готовность %",
    "feature2Text": "Смотрите свою готовность к экзамену в реальном времени.",
    "feature3Title": "Пояснения со ссылками на ПДД",
    "feature3Text": "Глубокое объяснение с юридической ссылкой в каждом ответе.",
    "feature4Title": "Реальная симуляция экзамена",
    "feature4Text": "20 вопросов, 25 минут, остановка на 3-й ошибке — точно как в реальности.",
    "howItWorksTitle": "Как это работает",
    "step1Title": "Зарегистрируйтесь",
    "step1Text": "С номером телефона за одну минуту.",
    "step2Title": "Решайте билет",
    "step2Text": "Первый билет всегда бесплатный.",
    "step3Title": "Следите за готовностью",
    "step3Text": "Смотрите слабые темы в статистике.",
    "faqTitle": "Вопросы и ответы",
    "faq1Q": "Первый билет правда бесплатный?",
    "faq1A": "Да, 1-й билет всегда бесплатный для каждого зарегистрированного пользователя.",
    "faq2Q": "Откуда взяты вопросы?",
    "faq2A": "Реальные экзаменационные вопросы из источника с подтверждённой лицензией.",
    "faq3Q": "На скольких языках работает?",
    "faq3A": "На узбекском (латиница и кириллица) и русском языках.",
    "footer": "AvtoTest — {year}"
  },
  "Dashboard": {
    "welcome": "Добро пожаловать,",
    "planPremium": "Премиум",
    "planFree": "Бесплатный план",
    "streakSuffix": "дней подряд",
    "streakToday": "Сегодня: {done}/{goal} вопросов",
    "readinessLabel": "Готовность",
    "navVariantsTitle": "Билеты",
    "navVariantsDesc": "61 билет, открываются последовательно",
    "navExamTitle": "Симуляция экзамена",
    "navExamDesc": "20 вопросов, 25 минут",
    "navPracticeTitle": "Практика",
    "navPracticeDesc": "По теме или по знаку",
    "navMistakesTitle": "Работа над ошибками",
    "navMistakesDesc": "Повторение на основе FSRS"
  },
  "ExamMockup": {
    "modeUnanswered": "Без ответа",
    "modeCorrect": "Дан правильный ответ",
    "modeIncorrect": "Дан неправильный ответ",
    "modeExamHidden": "Режим экзамена (обратная связь скрыта)",
    "positionLabel": "{n} / {total}"
  }
}
```

- [ ] **Step 5: Run typecheck to confirm the JSON files parse and next-intl plugin loads**

```bash
cd "/home/sher/Рабочий стол/avtotest/frontend" && npm run typecheck
```

Expected: clean (no code references these yet, so this just confirms `next.config.mjs`/JSON are syntactically valid via a subsequent build check in Step 9).

- [ ] **Step 6: Create middleware.ts (i18n routing only for now)**

`frontend/src/middleware.ts`:

```ts
import createMiddleware from "next-intl/middleware";
import { locales, defaultLocale } from "@/i18n/config";

export default createMiddleware({
  locales,
  defaultLocale,
  localePrefix: "always",
});

export const config = {
  matcher: ["/((?!api|_next|.*\\..*).*)"],
};
```

- [ ] **Step 7: Move and rewrite the root layout**

```bash
mkdir -p "/home/sher/Рабочий стол/avtotest/frontend/src/app/[locale]"
git -C "/home/sher/Рабочий стол/avtotest" rm frontend/src/app/layout.tsx
```

`frontend/src/app/[locale]/layout.tsx`:

```tsx
import type { Metadata } from "next";
import { Baloo_2, Manrope } from "next/font/google";
import { NextIntlClientProvider } from "next-intl";
import { getMessages } from "next-intl/server";
import { notFound } from "next/navigation";
import { Providers } from "@/app/providers";
import { ThemeToggle } from "@/components/theme-toggle";
import { locales, type Locale } from "@/i18n/config";
import "../globals.css";

const baloo = Baloo_2({ subsets: ["latin"], weight: ["600", "700", "800"], variable: "--font-baloo" });
const manrope = Manrope({ subsets: ["latin"], weight: ["400", "500", "600", "700"], variable: "--font-manrope" });

export const metadata: Metadata = {
  title: "AvtoTest",
  description: "Haydovchilik nazariy imtihoniga tayyorgarlik",
};

export default async function LocaleLayout({
  children,
  params: { locale },
}: {
  children: React.ReactNode;
  params: { locale: string };
}) {
  if (!locales.includes(locale as Locale)) notFound();
  const messages = await getMessages();

  return (
    <html lang={locale} suppressHydrationWarning className={`${baloo.variable} ${manrope.variable}`}>
      <body>
        <NextIntlClientProvider messages={messages}>
          <Providers>
            <div className="fixed right-4 top-4 z-50">
              <ThemeToggle />
            </div>
            {children}
          </Providers>
        </NextIntlClientProvider>
      </body>
    </html>
  );
}
```

- [ ] **Step 8: Move the 3 pages under `[locale]/` and wire translations**

```bash
cd "/home/sher/Рабочий стол/avtotest"
mkdir -p "frontend/src/app/[locale]/(public)" "frontend/src/app/[locale]/(app)/dashboard" "frontend/src/app/[locale]/(app)/exam-mockup"
git mv "frontend/src/app/(public)/page.tsx" "frontend/src/app/[locale]/(public)/page.tsx"
git mv "frontend/src/app/(public)/demo-question-block.tsx" "frontend/src/app/[locale]/(public)/demo-question-block.tsx"
git mv "frontend/src/app/(public)/page.test.tsx" "frontend/src/app/[locale]/(public)/page.test.tsx"
git mv "frontend/src/app/(app)/dashboard/page.tsx" "frontend/src/app/[locale]/(app)/dashboard/page.tsx"
git mv "frontend/src/app/(app)/dashboard/page.test.tsx" "frontend/src/app/[locale]/(app)/dashboard/page.test.tsx"
git mv "frontend/src/app/(app)/exam-mockup/page.tsx" "frontend/src/app/[locale]/(app)/exam-mockup/page.tsx"
git mv "frontend/src/app/(app)/exam-mockup/page.test.tsx" "frontend/src/app/[locale]/(app)/exam-mockup/page.test.tsx"
rmdir "frontend/src/app/(public)" "frontend/src/app/(app)/dashboard" "frontend/src/app/(app)/exam-mockup" "frontend/src/app/(app)" 2>/dev/null || true
```

Replace `frontend/src/app/[locale]/(public)/page.tsx` with:

```tsx
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { proofStats } from "@/lib/mock-data";
import { DemoQuestionBlock } from "./demo-question-block";

export default function LandingPage() {
  const t = useTranslations("Landing");

  const features = [
    { title: t("feature1Title"), text: t("feature1Text") },
    { title: t("feature2Title"), text: t("feature2Text") },
    { title: t("feature3Title"), text: t("feature3Text") },
    { title: t("feature4Title"), text: t("feature4Text") },
  ];
  const steps = [
    { n: 1, title: t("step1Title"), text: t("step1Text") },
    { n: 2, title: t("step2Title"), text: t("step2Text") },
    { n: 3, title: t("step3Title"), text: t("step3Text") },
  ];
  const faqs = [
    { q: t("faq1Q"), a: t("faq1A") },
    { q: t("faq2Q"), a: t("faq2A") },
    { q: t("faq3Q"), a: t("faq3A") },
  ];

  return (
    <main className="mx-auto max-w-5xl px-4 py-12">
      <section className="text-center">
        <h1 className="font-display text-4xl font-extrabold leading-tight md:text-6xl">
          {t("heroTitleBefore")} <span className="text-accent">{t("heroTitleAccent")}</span> {t("heroTitleAfter")}
        </h1>
        <p className="mx-auto mt-4 max-w-xl text-lg text-muted-foreground">{t("heroSubtitle")}</p>
        <div className="mt-8 flex flex-col items-center gap-3 sm:flex-row sm:justify-center">
          <Button variant="game" size="lg">
            {t("ctaStart")}
          </Button>
          <span className="rounded-full border border-border px-4 py-1 text-sm text-muted-foreground">
            {t("ctaNoSignup")}
          </span>
        </div>
      </section>

      <section className="mt-16">
        <h2 className="mb-6 text-center font-display text-2xl font-bold">{t("demoSectionTitle")}</h2>
        <DemoQuestionBlock />
      </section>

      <section className="mt-16 grid grid-cols-2 gap-6 text-center sm:grid-cols-4">
        {proofStats.map((s) => (
          <div key={s.label}>
            <p className="font-display text-3xl font-extrabold text-accent">{s.value}</p>
            <p className="text-sm text-muted-foreground">{s.label}</p>
          </div>
        ))}
      </section>

      <section className="mt-16">
        <h2 className="mb-6 text-center font-display text-2xl font-bold">{t("whyUsTitle")}</h2>
        <div className="grid gap-4 sm:grid-cols-2">
          {features.map((f) => (
            <div key={f.title} className="rounded-lg border border-border bg-card p-5">
              <h3 className="font-display font-bold">{f.title}</h3>
              <p className="mt-1 text-sm text-muted-foreground">{f.text}</p>
            </div>
          ))}
        </div>
      </section>

      <section className="mt-16">
        <h2 className="mb-6 text-center font-display text-2xl font-bold">{t("howItWorksTitle")}</h2>
        <div className="grid gap-6 sm:grid-cols-3">
          {steps.map((s) => (
            <div key={s.n} className="text-center">
              <div className="mx-auto mb-3 flex h-10 w-10 items-center justify-center rounded-full bg-accent font-display font-bold text-accent-foreground">
                {s.n}
              </div>
              <h3 className="font-display font-bold">{s.title}</h3>
              <p className="mt-1 text-sm text-muted-foreground">{s.text}</p>
            </div>
          ))}
        </div>
      </section>

      <section className="mt-16">
        <h2 className="mb-6 text-center font-display text-2xl font-bold">{t("faqTitle")}</h2>
        <div className="mx-auto max-w-2xl space-y-2">
          {faqs.map((f) => (
            <details key={f.q} className="rounded-lg border border-border bg-card p-4">
              <summary className="cursor-pointer font-semibold">{f.q}</summary>
              <p className="mt-2 text-sm text-muted-foreground">{f.a}</p>
            </details>
          ))}
        </div>
      </section>

      <footer className="mt-16 border-t border-border pt-6 text-center text-sm text-muted-foreground">
        {t("footer", { year: new Date().getFullYear() })}
      </footer>
    </main>
  );
}
```

`demo-question-block.tsx` needs no text changes (it has no hardcoded UI-chrome strings — only mock-data-sourced question/answer text, which stays as-is). Leave it byte-identical at its new path.

Replace `frontend/src/app/[locale]/(public)/page.test.tsx` with:

```tsx
import { render, screen } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { describe, it, expect } from "vitest";
import messages from "../../../../messages/uz-Latn.json";
import LandingPage from "./page";

function renderWithIntl() {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      <LandingPage />
    </NextIntlClientProvider>
  );
}

describe("LandingPage", () => {
  it("renders the hero CTA and all proof stats", () => {
    renderWithIntl();
    expect(screen.getByRole("button", { name: "Bepul boshlash" })).toBeInTheDocument();
    expect(screen.getByText("1235")).toBeInTheDocument();
    expect(screen.getByText("61")).toBeInTheDocument();
  });

  it("renders the interactive demo question", () => {
    renderWithIntl();
    expect(screen.getByText(/svetofor ishlamayapti/)).toBeInTheDocument();
  });
});
```

- [ ] **Step 9: Run the landing test, verify it passes**

Run: `cd "/home/sher/Рабочий стол/avtotest/frontend" && npm test -- "app/\[locale\]/(public)/page"`
Expected: 2 passed.

- [ ] **Step 10: Commit checkpoint (landing done), continue with dashboard + exam-mockup**

This task continues in the SAME commit — do not commit yet. Proceed to Step 11.

- [ ] **Step 11: Wire translations into the Dashboard page**

Replace `frontend/src/app/[locale]/(app)/dashboard/page.tsx` with:

```tsx
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { ResultRing } from "@/components/shared/result-ring";
import { mockProfile } from "@/lib/mock-data";
import { Flame } from "lucide-react";

export default function DashboardPage() {
  const t = useTranslations("Dashboard");
  const { name, isVip, streak, readinessPercent } = mockProfile;

  const navCards = [
    { title: t("navVariantsTitle"), desc: t("navVariantsDesc") },
    { title: t("navExamTitle"), desc: t("navExamDesc") },
    { title: t("navPracticeTitle"), desc: t("navPracticeDesc") },
    { title: t("navMistakesTitle"), desc: t("navMistakesDesc") },
  ];

  return (
    <main className="mx-auto max-w-4xl px-4 py-8">
      <header className="flex items-center justify-between">
        <div>
          <p className="text-muted-foreground">{t("welcome")}</p>
          <h1 className="font-display text-2xl font-bold">{name}</h1>
        </div>
        <span
          className={
            isVip
              ? "rounded-full bg-gold px-4 py-1 text-sm font-bold text-background"
              : "rounded-full border border-border px-4 py-1 text-sm text-muted-foreground"
          }
        >
          {isVip ? t("planPremium") : t("planFree")}
        </span>
      </header>

      <section className="mt-6 grid gap-4 sm:grid-cols-2">
        <div className="rounded-lg border border-border bg-card p-5">
          <div className="flex items-center gap-2">
            <Flame className="h-6 w-6 text-streak" />
            <span className="font-display text-2xl font-extrabold">{streak.current}</span>
            <span className="text-muted-foreground">{t("streakSuffix")}</span>
          </div>
          <div className="mt-3 h-2 w-full overflow-hidden rounded-full bg-border">
            <div
              className="h-full rounded-full bg-streak"
              style={{ width: `${Math.min(100, (streak.todayDone / streak.dailyGoal) * 100)}%` }}
            />
          </div>
          <p className="mt-1 text-sm text-muted-foreground">
            {t("streakToday", { done: streak.todayDone, goal: streak.dailyGoal })}
          </p>
        </div>

        <div className="flex items-center justify-center rounded-lg border border-border bg-card p-5">
          <ResultRing percent={readinessPercent} label={t("readinessLabel")} />
        </div>
      </section>

      <section className="mt-6 grid gap-4 sm:grid-cols-2">
        {navCards.map((c) => (
          <Button
            key={c.title}
            variant="game"
            className="h-auto flex-col items-start gap-1 whitespace-normal px-5 py-4 text-left"
          >
            <span className="font-display text-base font-bold">{c.title}</span>
            <span className="text-xs font-normal opacity-80">{c.desc}</span>
          </Button>
        ))}
      </section>
    </main>
  );
}
```

Replace `frontend/src/app/[locale]/(app)/dashboard/page.test.tsx` with:

```tsx
import { render, screen } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { describe, it, expect } from "vitest";
import messages from "../../../../../messages/uz-Latn.json";
import DashboardPage from "./page";

function renderWithIntl() {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      <DashboardPage />
    </NextIntlClientProvider>
  );
}

describe("DashboardPage", () => {
  it("renders the streak count and readiness ring", () => {
    renderWithIntl();
    expect(screen.getByText("12")).toBeInTheDocument();
    expect(screen.getByText("68%")).toBeInTheDocument();
  });

  it("renders all four navigation cards", () => {
    renderWithIntl();
    expect(screen.getByText("Biletlar")).toBeInTheDocument();
    expect(screen.getByText("Imtihon simulyatsiyasi")).toBeInTheDocument();
    expect(screen.getByText("Mashq")).toBeInTheDocument();
    expect(screen.getByText("Xatolar ustida ishlash")).toBeInTheDocument();
  });

  it("shows the free-plan badge when the user is not VIP", () => {
    renderWithIntl();
    expect(screen.getByText("Bepul reja")).toBeInTheDocument();
  });
});
```

- [ ] **Step 12: Run the dashboard test, verify it passes**

Run: `npm test -- dashboard/page`
Expected: 3 passed.

- [ ] **Step 13: Wire translations into the exam-mockup page**

Replace `frontend/src/app/[locale]/(app)/exam-mockup/page.tsx` with:

```tsx
"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { QuestionCard } from "@/components/shared/question-card";
import { AnswerOption, type AnswerState } from "@/components/shared/answer-option";
import { CountdownTimer } from "@/components/shared/countdown-timer";
import { Button } from "@/components/ui/button";
import { mockExamQuestions } from "@/lib/mock-data";

type Mode = "unanswered" | "correct" | "incorrect" | "exam-hidden";

export default function ExamMockupPage() {
  const t = useTranslations("ExamMockup");
  const [mode, setMode] = useState<Mode>("unanswered");
  const question = mockExamQuestions[0];
  const wrongAnswerId = question.answers.find((a) => a.id !== question.correctAnswerId)!.id;
  const selectedAnswerId =
    mode === "correct" ? question.correctAnswerId : mode === "incorrect" ? wrongAnswerId : null;

  const modeLabels: Record<Mode, string> = {
    unanswered: t("modeUnanswered"),
    correct: t("modeCorrect"),
    incorrect: t("modeIncorrect"),
    "exam-hidden": t("modeExamHidden"),
  };

  function stateFor(answerId: string): AnswerState {
    if (mode === "unanswered") return "neutral";
    if (mode === "exam-hidden") return answerId === selectedAnswerId ? "selected" : "neutral";
    if (answerId === question.correctAnswerId) return "correct";
    if (answerId === selectedAnswerId) return "incorrect";
    return "neutral";
  }

  return (
    <main className="mx-auto max-w-2xl px-4 py-8">
      {/* Mockup-only tooling: Phase B3 replaces this with real session state. */}
      <div className="mb-4 flex flex-wrap gap-2" role="group" aria-label="Mockup holatini tanlash">
        {(Object.keys(modeLabels) as Mode[]).map((m) => (
          <Button key={m} size="sm" variant={m === mode ? "default" : "outline"} onClick={() => setMode(m)}>
            {modeLabels[m]}
          </Button>
        ))}
      </div>

      <div className="mb-4 flex items-center justify-between">
        <span className="text-sm text-muted-foreground">{t("positionLabel", { n: 1, total: 20 })}</span>
        <CountdownTimer remainingSeconds={mode === "exam-hidden" ? 45 : 900} />
      </div>

      <QuestionCard questionNumber={1} totalQuestions={20} text={question.text} hasImage={question.hasImage} />

      <div className="mt-4 flex flex-col gap-3">
        {question.answers.map((a) => (
          <AnswerOption key={a.id} shortcutLabel={a.shortcutLabel} text={a.text} state={stateFor(a.id)} />
        ))}
      </div>

      {(mode === "correct" || mode === "incorrect") && (
        <div className="mt-4 rounded-lg border border-gold bg-gold/10 p-4">
          <p className="font-display font-bold text-gold">MUHIM</p>
          <p className="mt-1 text-sm">
            Svetofor ishlamagan chorrahada — YHQning tegishli qoidasiga ko&apos;ra o&apos;ngdan kelayotgan
            transport vositasi ustunlikka ega bo&apos;ladi.
          </p>
        </div>
      )}
    </main>
  );
}
```

Replace `frontend/src/app/[locale]/(app)/exam-mockup/page.test.tsx` with:

```tsx
import { render, screen, fireEvent } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { describe, it, expect } from "vitest";
import messages from "../../../../../messages/uz-Latn.json";
import ExamMockupPage from "./page";

function renderWithIntl() {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      <ExamMockupPage />
    </NextIntlClientProvider>
  );
}

describe("ExamMockupPage", () => {
  it("shows no correct/incorrect feedback in the unanswered state", () => {
    renderWithIntl();
    expect(screen.queryByTestId("answer-correct-icon")).not.toBeInTheDocument();
    expect(screen.queryByTestId("answer-incorrect-icon")).not.toBeInTheDocument();
  });

  it("reveals the correct answer and explanation when switched to the correct state", () => {
    renderWithIntl();
    fireEvent.click(screen.getByRole("button", { name: "To'g'ri javob berilgan" }));
    expect(screen.getByTestId("answer-correct-icon")).toBeInTheDocument();
    expect(screen.getByText("MUHIM")).toBeInTheDocument();
  });

  it("never reveals correctness in the exam-hidden state, even after selecting an answer", () => {
    renderWithIntl();
    fireEvent.click(screen.getByRole("button", { name: "Imtihon rejimi (feedback yashirin)" }));
    expect(screen.queryByTestId("answer-correct-icon")).not.toBeInTheDocument();
    expect(screen.queryByTestId("answer-incorrect-icon")).not.toBeInTheDocument();
  });

  it("shows a gold timer normally and a pulsating red timer in the exam-hidden (low-time) state", () => {
    renderWithIntl();
    expect(screen.getByTestId("countdown-timer").className).toContain("text-gold");
    fireEvent.click(screen.getByRole("button", { name: "Imtihon rejimi (feedback yashirin)" }));
    expect(screen.getByTestId("countdown-timer").className).toContain("text-danger");
  });
});
```

- [ ] **Step 14: Add the ThemeToggle translations**

Modify `frontend/src/components/theme-toggle.tsx` — replace the hardcoded `aria-label` strings:

```tsx
"use client";

import { useEffect, useState } from "react";
import { useTheme } from "next-themes";
import { useTranslations } from "next-intl";
import { Moon, Sun } from "lucide-react";

export function ThemeToggle() {
  const { theme, setTheme } = useTheme();
  const t = useTranslations("ThemeToggle");
  const [mounted, setMounted] = useState(false);

  useEffect(() => {
    setMounted(true);
  }, []);

  if (!mounted) {
    return <span className="inline-block h-9 w-9" aria-hidden />;
  }

  const isDark = theme === "dark";

  return (
    <button
      type="button"
      aria-label={isDark ? t("toLight") : t("toDark")}
      onClick={() => setTheme(isDark ? "light" : "dark")}
      className="flex h-9 w-9 items-center justify-center rounded-full border border-border bg-card"
    >
      {isDark ? (
        <Sun data-testid="theme-toggle-sun" className="h-4 w-4" />
      ) : (
        <Moon data-testid="theme-toggle-moon" className="h-4 w-4" />
      )}
    </button>
  );
}
```

`useTranslations` inside a component NOT wrapped in `NextIntlClientProvider` at test time will throw — update `frontend/src/components/theme-toggle.test.tsx`'s `renderWithTheme` helper to also wrap with `NextIntlClientProvider`:

```tsx
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { ThemeProvider } from "next-themes";
import { NextIntlClientProvider } from "next-intl";
import messages from "../../messages/uz-Latn.json";
import { ThemeToggle } from "./theme-toggle";

function renderWithTheme() {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      <ThemeProvider attribute="class" defaultTheme="dark" enableSystem={false}>
        <ThemeToggle />
      </ThemeProvider>
    </NextIntlClientProvider>
  );
}

describe("ThemeToggle", () => {
  it("shows the sun icon (offering to switch to light) while the theme is dark", async () => {
    renderWithTheme();
    await waitFor(() => expect(screen.getByTestId("theme-toggle-sun")).toBeInTheDocument());
  });

  it("switches to the moon icon after being clicked", async () => {
    renderWithTheme();
    await waitFor(() => expect(screen.getByRole("button")).toBeInTheDocument());
    fireEvent.click(screen.getByRole("button"));
    await waitFor(() => expect(screen.getByTestId("theme-toggle-moon")).toBeInTheDocument());
  });
});
```

- [ ] **Step 15: Run the full suite, typecheck, and build**

```bash
cd "/home/sher/Рабочий стол/avtotest/frontend" && npm run typecheck && npm test && npm run build
```

Expected: all clean. Build's route table now shows `/[locale]`, `/[locale]/dashboard`, `/[locale]/exam-mockup` (or their resolved equivalents) instead of the old bare paths — confirm no leftover `src/app/(public)` or `src/app/(app)` or `src/app/layout.tsx` remain (`find frontend/src/app -maxdepth 2` should show only `[locale]`, `providers.tsx`, and no bare `layout.tsx`/`(public)`/`(app)`).

- [ ] **Step 16: Commit**

```bash
cd "/home/sher/Рабочий стол/avtotest" && git add frontend/ && git commit -m "feat(frontend): next-intl setup, locale-route all pages, translate UI chrome to 3 locales"
```

---

### Task 2: Backend fetch helper + auth cookie helpers + single-flight refresh lock

**Files:**
- Create: `frontend/src/lib/backend.ts`
- Create: `frontend/src/lib/auth-cookies.ts`
- Create: `frontend/src/lib/refresh-lock.ts`
- Create: `frontend/src/lib/refresh-lock.test.ts`

**Interfaces:**
- Consumes: nothing new.
- Produces: `backendFetch(path: string, init?: RequestInit): Promise<Response>`; `AUTH_COOKIE = "at"`, `REFRESH_COOKIE = "rt"`; `setAuthCookies(res: NextResponse, tokens: {accessToken: string, refreshToken: string}): void`; `clearAuthCookies(res: NextResponse): void`; `readCookie(request: Request, name: string): string | undefined`; `refreshOnce(refreshToken: string, doRefresh: (rt: string) => Promise<{accessToken: string, refreshToken: string} | null>): Promise<{accessToken: string, refreshToken: string} | null>` — every Route Handler task (3-6) imports these.

- [ ] **Step 1: Write the failing refresh-lock test**

`frontend/src/lib/refresh-lock.test.ts`:

```ts
import { describe, it, expect, vi } from "vitest";
import { refreshOnce } from "./refresh-lock";

describe("refreshOnce", () => {
  it("only calls doRefresh once for concurrent callers with the same token", async () => {
    let resolveRefresh: (v: { accessToken: string; refreshToken: string }) => void;
    const doRefresh = vi.fn(
      () =>
        new Promise<{ accessToken: string; refreshToken: string } | null>((resolve) => {
          resolveRefresh = resolve;
        })
    );

    const call1 = refreshOnce("rt-1", doRefresh);
    const call2 = refreshOnce("rt-1", doRefresh);
    resolveRefresh!({ accessToken: "new-at", refreshToken: "new-rt" });

    const [result1, result2] = await Promise.all([call1, call2]);

    expect(doRefresh).toHaveBeenCalledTimes(1);
    expect(result1).toEqual({ accessToken: "new-at", refreshToken: "new-rt" });
    expect(result2).toEqual({ accessToken: "new-at", refreshToken: "new-rt" });
  });

  it("allows a new refresh after the previous one settles", async () => {
    const doRefresh = vi
      .fn()
      .mockResolvedValueOnce({ accessToken: "at-1", refreshToken: "rt-1" })
      .mockResolvedValueOnce({ accessToken: "at-2", refreshToken: "rt-2" });

    const first = await refreshOnce("rt-0", doRefresh);
    const second = await refreshOnce("rt-1", doRefresh);

    expect(doRefresh).toHaveBeenCalledTimes(2);
    expect(first?.accessToken).toBe("at-1");
    expect(second?.accessToken).toBe("at-2");
  });

  it("propagates a null result (refresh failed) to all concurrent callers", async () => {
    const doRefresh = vi.fn().mockResolvedValue(null);
    const [result1, result2] = await Promise.all([refreshOnce("rt-x", doRefresh), refreshOnce("rt-x", doRefresh)]);
    expect(doRefresh).toHaveBeenCalledTimes(1);
    expect(result1).toBeNull();
    expect(result2).toBeNull();
  });
});
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd "/home/sher/Рабочий стол/avtotest/frontend" && npm test -- refresh-lock`
Expected: FAIL — module not found.

- [ ] **Step 3: Implement the three lib modules**

`frontend/src/lib/backend.ts`:

```ts
const BACKEND_URL = process.env.BACKEND_URL ?? "http://localhost:8090";

export function backendFetch(path: string, init?: RequestInit): Promise<Response> {
  return fetch(`${BACKEND_URL}/api/v1${path}`, init);
}
```

`frontend/src/lib/auth-cookies.ts`:

```ts
import type { NextResponse } from "next/server";

export const AUTH_COOKIE = "at";
export const REFRESH_COOKIE = "rt";

const AT_MAX_AGE = 900; // 15 minutes, matches backend access-token TTL
const RT_MAX_AGE = 60 * 60 * 24 * 30; // 30 days, matches backend rotating refresh TTL

const baseOptions = {
  httpOnly: true,
  sameSite: "lax" as const,
  secure: process.env.NODE_ENV === "production",
  path: "/",
};

export function setAuthCookies(
  res: NextResponse,
  tokens: { accessToken: string; refreshToken: string }
): void {
  res.cookies.set(AUTH_COOKIE, tokens.accessToken, { ...baseOptions, maxAge: AT_MAX_AGE });
  res.cookies.set(REFRESH_COOKIE, tokens.refreshToken, { ...baseOptions, maxAge: RT_MAX_AGE });
}

export function clearAuthCookies(res: NextResponse): void {
  res.cookies.set(AUTH_COOKIE, "", { ...baseOptions, maxAge: 0 });
  res.cookies.set(REFRESH_COOKIE, "", { ...baseOptions, maxAge: 0 });
}

// Reads a cookie directly from the request's Cookie header rather than via
// next/headers' cookies() — that API depends on Next's request-scoped
// AsyncLocalStorage context, which doesn't exist when a Route Handler is
// unit-tested by importing and calling it directly. Reading the raw header
// works identically in production (Route Handlers always receive the real
// Cookie header) and needs no Next-runtime context to test.
export function readCookie(request: Request, name: string): string | undefined {
  const header = request.headers.get("cookie");
  if (!header) return undefined;
  const match = header
    .split(";")
    .map((c) => c.trim())
    .find((c) => c.startsWith(`${name}=`));
  return match ? decodeURIComponent(match.slice(name.length + 1)) : undefined;
}
```

`frontend/src/lib/refresh-lock.ts`:

```ts
export interface RefreshedTokens {
  accessToken: string;
  refreshToken: string;
}

let inFlight: Promise<RefreshedTokens | null> | null = null;

// Single-flight: concurrent callers share the SAME in-flight refresh call
// rather than each triggering their own — the backend rotates+revokes
// refresh tokens per use, so two concurrent refresh calls with the same
// token would make the backend treat the second as replay/theft and
// revoke ALL of the user's sessions (this exact bug happened in the
// Flutter-era AuthInterceptor before its single-flight fix).
export function refreshOnce(
  refreshToken: string,
  doRefresh: (rt: string) => Promise<RefreshedTokens | null>
): Promise<RefreshedTokens | null> {
  if (!inFlight) {
    inFlight = doRefresh(refreshToken).finally(() => {
      inFlight = null;
    });
  }
  return inFlight;
}
```

- [ ] **Step 4: Run to verify it passes, then typecheck**

```bash
npm test -- refresh-lock && npm run typecheck
```

Expected: 3 passed, typecheck clean.

- [ ] **Step 5: Commit**

```bash
cd "/home/sher/Рабочий стол/avtotest" && git add frontend/ && git commit -m "feat(frontend): backend fetch helper, auth cookie helpers, single-flight refresh lock"
```

---

### Task 3: OTP request + verify route handlers

**Files:**
- Create: `frontend/src/app/api/auth/otp/request/route.ts`
- Create: `frontend/src/app/api/auth/otp/request/route.test.ts`
- Create: `frontend/src/app/api/auth/otp/verify/route.ts`
- Create: `frontend/src/app/api/auth/otp/verify/route.test.ts`

**Interfaces:**
- Consumes: `backendFetch` (Task 2), `setAuthCookies`/`AUTH_COOKIE`/`REFRESH_COOKIE` (Task 2).
- Produces: `POST /api/auth/otp/request` and `POST /api/auth/otp/verify` — Task 7/8 UI pages call these by URL (no shared TS import needed, they're HTTP endpoints).

- [ ] **Step 1: Write the failing otp/request test**

`frontend/src/app/api/auth/otp/request/route.test.ts`:

```ts
import { describe, it, expect, vi, afterEach } from "vitest";
import { POST } from "./route";

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("POST /api/auth/otp/request", () => {
  it("forwards the request body to the backend and returns its response verbatim", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValue(
        new Response(JSON.stringify({ data: { channel: "sandbox", debug_code: "123456" } }), { status: 200 })
      );
    vi.stubGlobal("fetch", fetchMock);

    const request = new Request("http://localhost/api/auth/otp/request", {
      method: "POST",
      body: JSON.stringify({ phone: "901112233" }),
    });
    const response = await POST(request);
    const json = await response.json();

    expect(fetchMock).toHaveBeenCalledWith(
      "http://localhost:8090/api/v1/auth/otp/request",
      expect.objectContaining({ method: "POST" })
    );
    expect(response.status).toBe(200);
    expect(json).toEqual({ data: { channel: "sandbox", debug_code: "123456" } });
  });

  it("passes through a rate_limited error with its status code", async () => {
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockResolvedValue(
          new Response(JSON.stringify({ error: { code: "rate_limited", message: "too many requests" } }), {
            status: 429,
          })
        )
    );

    const request = new Request("http://localhost/api/auth/otp/request", {
      method: "POST",
      body: JSON.stringify({ phone: "901112233" }),
    });
    const response = await POST(request);

    expect(response.status).toBe(429);
    expect((await response.json()).error.code).toBe("rate_limited");
  });
});
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd "/home/sher/Рабочий стол/avtotest/frontend" && npm test -- "otp/request"`
Expected: FAIL — module not found.

- [ ] **Step 3: Implement otp/request**

`frontend/src/app/api/auth/otp/request/route.ts`:

```ts
import { NextResponse } from "next/server";
import { backendFetch } from "@/lib/backend";

export async function POST(request: Request) {
  const body = await request.json();
  const backendRes = await backendFetch("/auth/otp/request", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  const data = await backendRes.json();
  return NextResponse.json(data, { status: backendRes.status });
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `npm test -- "otp/request"`
Expected: 2 passed.

- [ ] **Step 5: Write the failing otp/verify test**

`frontend/src/app/api/auth/otp/verify/route.test.ts`:

```ts
import { describe, it, expect, vi, afterEach } from "vitest";
import { POST } from "./route";
import { AUTH_COOKIE, REFRESH_COOKIE } from "@/lib/auth-cookies";

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("POST /api/auth/otp/verify", () => {
  it("sets httpOnly auth cookies and never exposes tokens in the response body", async () => {
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockResolvedValue(
          new Response(JSON.stringify({ data: { access_token: "abc.def", refresh_token: "xyz.123" } }), {
            status: 200,
          })
        )
    );

    const request = new Request("http://localhost/api/auth/otp/verify", {
      method: "POST",
      body: JSON.stringify({ phone: "901112233", code: "123456" }),
    });
    const response = await POST(request);
    const json = await response.json();

    expect(json).toEqual({ data: { ok: true } });
    expect(JSON.stringify(json)).not.toContain("abc.def");
    expect(JSON.stringify(json)).not.toContain("xyz.123");

    const atCookie = response.cookies.get(AUTH_COOKIE);
    const rtCookie = response.cookies.get(REFRESH_COOKIE);
    expect(atCookie?.value).toBe("abc.def");
    expect(atCookie?.httpOnly).toBe(true);
    expect(rtCookie?.value).toBe("xyz.123");
    expect(rtCookie?.httpOnly).toBe(true);
  });

  it("passes through invalid_code without setting any cookies", async () => {
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockResolvedValue(
          new Response(JSON.stringify({ error: { code: "invalid_code", message: "wrong code" } }), { status: 400 })
        )
    );

    const request = new Request("http://localhost/api/auth/otp/verify", {
      method: "POST",
      body: JSON.stringify({ phone: "901112233", code: "000000" }),
    });
    const response = await POST(request);
    const json = await response.json();

    expect(response.status).toBe(400);
    expect(json.error.code).toBe("invalid_code");
    expect(response.cookies.get(AUTH_COOKIE)).toBeUndefined();
  });
});
```

- [ ] **Step 6: Run to verify it fails, then implement**

Run: `npm test -- "otp/verify"` → FAIL (module not found).

`frontend/src/app/api/auth/otp/verify/route.ts`:

```ts
import { NextResponse } from "next/server";
import { backendFetch } from "@/lib/backend";
import { setAuthCookies } from "@/lib/auth-cookies";

export async function POST(request: Request) {
  const body = await request.json();
  const backendRes = await backendFetch("/auth/otp/verify", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  const data = await backendRes.json();

  if (!backendRes.ok) {
    return NextResponse.json(data, { status: backendRes.status });
  }

  const response = NextResponse.json({ data: { ok: true } }, { status: 200 });
  setAuthCookies(response, {
    accessToken: data.data.access_token,
    refreshToken: data.data.refresh_token,
  });
  return response;
}
```

- [ ] **Step 7: Run to verify it passes, then full suite**

```bash
npm test -- "otp/verify" && npm run typecheck && npm test
```

Expected: all clean.

- [ ] **Step 8: Commit**

```bash
cd "/home/sher/Рабочий стол/avtotest" && git add frontend/ && git commit -m "feat(frontend): OTP request/verify route handlers (httpOnly cookie session)"
```

---

### Task 4: Refresh + logout route handlers

**Files:**
- Create: `frontend/src/lib/backend-refresh.ts`
- Create: `frontend/src/app/api/auth/refresh/route.ts`
- Create: `frontend/src/app/api/auth/refresh/route.test.ts`
- Create: `frontend/src/app/api/auth/logout/route.ts`
- Create: `frontend/src/app/api/auth/logout/route.test.ts`

**Interfaces:**
- Consumes: `backendFetch`, `setAuthCookies`, `clearAuthCookies`, `readCookie`, `AUTH_COOKIE`, `REFRESH_COOKIE` (Task 2), `refreshOnce` (Task 2).
- Produces: `callBackendRefresh(refreshToken: string): Promise<RefreshedTokens | null>` (from `lib/backend-refresh.ts`) — Task 5's proxy route reuses this same function (not a second copy) to call `refreshOnce`. `POST /api/auth/refresh`, `POST /api/auth/logout` HTTP endpoints.

- [ ] **Step 1: Write the failing refresh route test**

`frontend/src/app/api/auth/refresh/route.test.ts`:

```ts
import { describe, it, expect, vi, afterEach } from "vitest";
import { POST } from "./route";
import { AUTH_COOKIE, REFRESH_COOKIE } from "@/lib/auth-cookies";

afterEach(() => {
  vi.unstubAllGlobals();
});

function requestWithCookie(cookieHeader?: string): Request {
  const headers = cookieHeader ? { Cookie: cookieHeader } : {};
  return new Request("http://localhost/api/auth/refresh", { method: "POST", headers });
}

describe("POST /api/auth/refresh", () => {
  it("rotates cookies on a successful backend refresh", async () => {
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockResolvedValue(
          new Response(JSON.stringify({ data: { access_token: "new-at", refresh_token: "new-rt" } }), { status: 200 })
        )
    );

    const response = await POST(requestWithCookie("rt=old-rt"));

    expect(response.status).toBe(200);
    expect(response.cookies.get(AUTH_COOKIE)?.value).toBe("new-at");
    expect(response.cookies.get(REFRESH_COOKIE)?.value).toBe("new-rt");
  });

  it("returns invalid_refresh and clears cookies when no refresh cookie is present", async () => {
    const response = await POST(requestWithCookie(undefined));
    const json = await response.json();

    expect(response.status).toBe(401);
    expect(json.error.code).toBe("invalid_refresh");
    expect(response.cookies.get(AUTH_COOKIE)?.value).toBe("");
  });

  it("returns invalid_refresh and clears cookies when the backend rejects the refresh token", async () => {
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockResolvedValue(new Response(JSON.stringify({ error: { code: "refresh_reused" } }), { status: 401 }))
    );

    const response = await POST(requestWithCookie("rt=stolen-rt"));
    const json = await response.json();

    expect(response.status).toBe(401);
    expect(json.error.code).toBe("invalid_refresh");
    expect(response.cookies.get(REFRESH_COOKIE)?.value).toBe("");
  });
});
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd "/home/sher/Рабочий стол/avtotest/frontend" && npm test -- "auth/refresh"`
Expected: FAIL — module not found.

- [ ] **Step 3: Implement the shared refresh-caller and the refresh route**

`frontend/src/lib/backend-refresh.ts`:

```ts
import { backendFetch } from "@/lib/backend";
import type { RefreshedTokens } from "@/lib/refresh-lock";

export async function callBackendRefresh(refreshToken: string): Promise<RefreshedTokens | null> {
  const res = await backendFetch("/auth/refresh", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ refresh_token: refreshToken }),
  });
  if (!res.ok) return null;
  const data = await res.json();
  return { accessToken: data.data.access_token, refreshToken: data.data.refresh_token };
}
```

`frontend/src/app/api/auth/refresh/route.ts`:

```ts
import { NextResponse } from "next/server";
import { setAuthCookies, clearAuthCookies, readCookie, REFRESH_COOKIE } from "@/lib/auth-cookies";
import { refreshOnce } from "@/lib/refresh-lock";
import { callBackendRefresh } from "@/lib/backend-refresh";

export async function POST(request: Request) {
  const refreshToken = readCookie(request, REFRESH_COOKIE);
  if (!refreshToken) {
    const response = NextResponse.json(
      { error: { code: "invalid_refresh", message: "no refresh token" } },
      { status: 401 }
    );
    clearAuthCookies(response);
    return response;
  }

  const tokens = await refreshOnce(refreshToken, callBackendRefresh);
  if (!tokens) {
    const response = NextResponse.json(
      { error: { code: "invalid_refresh", message: "refresh failed" } },
      { status: 401 }
    );
    clearAuthCookies(response);
    return response;
  }

  const response = NextResponse.json({ data: { ok: true } }, { status: 200 });
  setAuthCookies(response, tokens);
  return response;
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `npm test -- "auth/refresh"`
Expected: 3 passed.

- [ ] **Step 5: Write the failing logout test**

`frontend/src/app/api/auth/logout/route.test.ts`:

```ts
import { describe, it, expect, vi, afterEach } from "vitest";
import { POST } from "./route";
import { AUTH_COOKIE, REFRESH_COOKIE } from "@/lib/auth-cookies";

afterEach(() => {
  vi.unstubAllGlobals();
});

function requestWithCookie(cookieHeader?: string): Request {
  const headers = cookieHeader ? { Cookie: cookieHeader } : {};
  return new Request("http://localhost/api/auth/logout", { method: "POST", headers });
}

describe("POST /api/auth/logout", () => {
  it("clears cookies when the backend call succeeds", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ ok: true }), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);

    const response = await POST(requestWithCookie("rt=some-rt; at=some-at"));

    expect(fetchMock).toHaveBeenCalledWith(
      "http://localhost:8090/api/v1/auth/logout",
      expect.objectContaining({ method: "POST" })
    );
    expect(response.cookies.get(AUTH_COOKIE)?.value).toBe("");
    expect(response.cookies.get(REFRESH_COOKIE)?.value).toBe("");
  });

  it("still clears cookies when the backend call throws (network error)", async () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new Error("network down")));

    const response = await POST(requestWithCookie("rt=some-rt; at=some-at"));

    expect(response.cookies.get(AUTH_COOKIE)?.value).toBe("");
    expect(response.cookies.get(REFRESH_COOKIE)?.value).toBe("");
  });

  it("clears cookies even when there was no refresh token to send", async () => {
    const response = await POST(requestWithCookie(undefined));
    expect(response.cookies.get(AUTH_COOKIE)?.value).toBe("");
    expect(response.cookies.get(REFRESH_COOKIE)?.value).toBe("");
  });
});
```

- [ ] **Step 6: Run to verify it fails, then implement**

Run: `npm test -- "auth/logout"` → FAIL (module not found).

`frontend/src/app/api/auth/logout/route.ts`:

```ts
import { NextResponse } from "next/server";
import { backendFetch } from "@/lib/backend";
import { clearAuthCookies, readCookie, REFRESH_COOKIE } from "@/lib/auth-cookies";

// Cookies are cleared unconditionally — logout must never leave the client
// "logged in" locally just because the backend call failed or the refresh
// token was already gone (mirrors the Flutter-era logout() precedent: it
// clears tokens on both a thrown exception and a Result.err).
export async function POST(request: Request) {
  const refreshToken = readCookie(request, REFRESH_COOKIE);
  if (refreshToken) {
    try {
      await backendFetch("/auth/logout", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ refresh_token: refreshToken }),
      });
    } catch {
      // Ignored deliberately — cookies are cleared below regardless.
    }
  }

  const response = NextResponse.json({ data: { ok: true } }, { status: 200 });
  clearAuthCookies(response);
  return response;
}
```

- [ ] **Step 7: Run to verify it passes, then full suite**

```bash
npm test -- "auth/logout" && npm run typecheck && npm test
```

Expected: all clean.

- [ ] **Step 8: Commit**

```bash
cd "/home/sher/Рабочий стол/avtotest" && git add frontend/ && git commit -m "feat(frontend): refresh (single-flight) and logout route handlers"
```

---

### Task 5: Generic authenticated proxy route (`/api/proxy/[...path]`)

**Files:**
- Create: `frontend/src/app/api/proxy/[...path]/route.ts`
- Create: `frontend/src/app/api/proxy/[...path]/route.test.ts`

**Interfaces:**
- Consumes: `backendFetch`, `setAuthCookies`, `clearAuthCookies`, `readCookie`, `AUTH_COOKIE`, `REFRESH_COOKIE` (Task 2), `refreshOnce` (Task 2), `callBackendRefresh` (Task 4).
- Produces: `GET/POST/PATCH/DELETE /api/proxy/*` — Phase B2's TanStack Query hooks will call these paths for every authenticated backend resource.

- [ ] **Step 1: Write the failing proxy tests**

`frontend/src/app/api/proxy/[...path]/route.test.ts`:

```ts
import { describe, it, expect, vi, afterEach } from "vitest";
import { GET, POST } from "./route";
import { AUTH_COOKIE, REFRESH_COOKIE } from "@/lib/auth-cookies";

afterEach(() => {
  vi.unstubAllGlobals();
});

function requestWithCookies(cookieHeader: string, init: RequestInit = {}): Request {
  const headers = new Headers(init.headers);
  headers.set("Cookie", cookieHeader);
  return new Request("http://localhost/api/proxy/x", { ...init, headers });
}

describe("proxy route", () => {
  it("returns 401 with no backend call when there is no access token cookie", async () => {
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);

    const response = await GET(requestWithCookies(""), { params: { path: ["me"] } });

    expect(response.status).toBe(401);
    expect((await response.json()).error.code).toBe("unauthorized");
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("forwards a GET with the Bearer token and returns the backend response verbatim", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValue(new Response(JSON.stringify({ data: { id: "profile-1" } }), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);

    const response = await GET(requestWithCookies("at=good-token"), { params: { path: ["me"] } });
    const json = await response.json();

    expect(fetchMock).toHaveBeenCalledWith(
      "http://localhost:8090/api/v1/me",
      expect.objectContaining({
        method: "GET",
        headers: expect.objectContaining({ Authorization: "Bearer good-token" }),
      })
    );
    expect(json).toEqual({ data: { id: "profile-1" } });
  });

  it("on a 401, refreshes once and retries the same request with the new token", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(new Response(JSON.stringify({ error: { code: "unauthorized" } }), { status: 401 }))
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ data: { access_token: "fresh-at", refresh_token: "fresh-rt" } }), {
          status: 200,
        })
      )
      .mockResolvedValueOnce(new Response(JSON.stringify({ data: { id: "profile-1" } }), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);

    const response = await GET(requestWithCookies("at=expired-token; rt=valid-rt"), { params: { path: ["me"] } });
    const json = await response.json();

    expect(fetchMock).toHaveBeenCalledTimes(3);
    expect(fetchMock).toHaveBeenNthCalledWith(
      3,
      "http://localhost:8090/api/v1/me",
      expect.objectContaining({ headers: expect.objectContaining({ Authorization: "Bearer fresh-at" }) })
    );
    expect(json).toEqual({ data: { id: "profile-1" } });
    expect(response.cookies.get(AUTH_COOKIE)?.value).toBe("fresh-at");
    expect(response.cookies.get(REFRESH_COOKIE)?.value).toBe("fresh-rt");
  });

  it("clears cookies and returns 401 when the refresh itself fails", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(new Response(JSON.stringify({ error: { code: "unauthorized" } }), { status: 401 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ error: { code: "refresh_reused" } }), { status: 401 }));
    vi.stubGlobal("fetch", fetchMock);

    const response = await GET(requestWithCookies("at=expired-token; rt=stolen-rt"), { params: { path: ["me"] } });
    const json = await response.json();

    expect(response.status).toBe(401);
    expect(json.error.code).toBe("unauthorized");
    expect(response.cookies.get(AUTH_COOKIE)?.value).toBe("");
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });

  it("forwards a POST body correctly, reusing the exact same body across the retry", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(new Response(JSON.stringify({ error: { code: "unauthorized" } }), { status: 401 }))
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ data: { access_token: "fresh-at", refresh_token: "fresh-rt" } }), {
          status: 200,
        })
      )
      .mockResolvedValueOnce(new Response(JSON.stringify({ data: { ok: true } }), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);

    const body = JSON.stringify({ question_id: "q1", answer_id: "a1" });
    await POST(requestWithCookies("at=expired-token; rt=valid-rt", { method: "POST", body }), {
      params: { path: ["sessions", "abc", "answers"] },
    });

    expect(fetchMock).toHaveBeenNthCalledWith(
      1,
      "http://localhost:8090/api/v1/sessions/abc/answers",
      expect.objectContaining({ method: "POST", body })
    );
    expect(fetchMock).toHaveBeenNthCalledWith(
      3,
      "http://localhost:8090/api/v1/sessions/abc/answers",
      expect.objectContaining({ method: "POST", body })
    );
  });
});
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd "/home/sher/Рабочий стол/avtotest/frontend" && npm test -- "api/proxy"`
Expected: FAIL — module not found.

- [ ] **Step 3: Implement the proxy route**

`frontend/src/app/api/proxy/[...path]/route.ts`:

```ts
import { NextResponse } from "next/server";
import { backendFetch } from "@/lib/backend";
import { setAuthCookies, clearAuthCookies, readCookie, AUTH_COOKIE, REFRESH_COOKIE } from "@/lib/auth-cookies";
import { refreshOnce } from "@/lib/refresh-lock";
import { callBackendRefresh } from "@/lib/backend-refresh";

async function forward(
  request: Request,
  path: string[],
  accessToken: string,
  body: string | undefined
): Promise<Response> {
  const url = new URL(request.url);
  const targetPath = `/${path.join("/")}${url.search}`;
  const init: RequestInit = {
    method: request.method,
    headers: {
      Authorization: `Bearer ${accessToken}`,
      "Content-Type": request.headers.get("content-type") ?? "application/json",
    },
  };
  if (body !== undefined) {
    init.body = body;
  }
  return backendFetch(targetPath, init);
}

async function handle(request: Request, context: { params: { path: string[] } }): Promise<Response> {
  const { path } = context.params;
  const accessToken = readCookie(request, AUTH_COOKIE);

  if (!accessToken) {
    return NextResponse.json({ error: { code: "unauthorized", message: "no access token" } }, { status: 401 });
  }

  // Read the body ONCE — a Request's body stream can only be consumed a
  // single time, and this same body must be reused verbatim on the retry
  // after a refresh (re-reading request.text() a second time would throw).
  const body = request.method !== "GET" && request.method !== "HEAD" ? await request.text() : undefined;

  let backendRes = await forward(request, path, accessToken, body);

  if (backendRes.status === 401) {
    const refreshToken = readCookie(request, REFRESH_COOKIE);
    const tokens = refreshToken ? await refreshOnce(refreshToken, callBackendRefresh) : null;

    if (!tokens) {
      const response = NextResponse.json(
        { error: { code: "unauthorized", message: "session expired" } },
        { status: 401 }
      );
      clearAuthCookies(response);
      return response;
    }

    backendRes = await forward(request, path, tokens.accessToken, body);
    const data = await backendRes.json();
    const response = NextResponse.json(data, { status: backendRes.status });
    setAuthCookies(response, tokens);
    return response;
  }

  const data = await backendRes.json();
  return NextResponse.json(data, { status: backendRes.status });
}

export const GET = handle;
export const POST = handle;
export const PATCH = handle;
export const DELETE = handle;
```

- [ ] **Step 4: Run to verify it passes, then full suite**

```bash
npm test -- "api/proxy" && npm run typecheck && npm test
```

Expected: all clean.

- [ ] **Step 5: Commit**

```bash
cd "/home/sher/Рабочий стол/avtotest" && git add frontend/ && git commit -m "feat(frontend): generic authenticated proxy route with single-flight 401 refresh-and-retry"
```

---

### Task 6: middleware.ts — add the auth-guard layer

**Files:**
- Modify: `frontend/src/middleware.ts`
- Create: `frontend/src/middleware.test.ts`

**Interfaces:**
- Consumes: `locales`, `defaultLocale` (Task 1), `AUTH_COOKIE`, `REFRESH_COOKIE` (Task 2).
- Produces: the composed default-exported `middleware(request: NextRequest): NextResponse` — no other task depends on this directly (it's the request-entry gate); Task 7/8 UI pages rely on its redirect behavior being correct.

- [ ] **Step 1: Write the failing middleware tests**

`frontend/src/middleware.test.ts`:

```ts
// @vitest-environment node
import { describe, it, expect, vi } from "vitest";
import { NextRequest, NextResponse } from "next/server";

vi.mock("next-intl/middleware", () => ({
  default: () => () => NextResponse.next(),
}));

import middleware from "./middleware";
import { AUTH_COOKIE } from "@/lib/auth-cookies";

function makeRequest(pathname: string, cookieHeader?: string): NextRequest {
  const headers = new Headers();
  if (cookieHeader) headers.set("cookie", cookieHeader);
  return new NextRequest(`http://localhost:3000${pathname}`, { headers });
}

describe("middleware auth guard", () => {
  it("redirects to login when a protected page is requested without a session cookie", () => {
    const response = middleware(makeRequest("/uz-Latn/dashboard"));
    expect(response.status).toBe(307);
    expect(response.headers.get("location")).toBe("http://localhost:3000/uz-Latn/login");
  });

  it("does not redirect a protected page when the access-token cookie is present", () => {
    const response = middleware(makeRequest("/uz-Latn/dashboard", `${AUTH_COOKIE}=some-token`));
    expect(response.headers.get("location")).toBeNull();
  });

  it("redirects an already-logged-in user away from the login page", () => {
    const response = middleware(makeRequest("/uz-Latn/login", `${AUTH_COOKIE}=some-token`));
    expect(response.status).toBe(307);
    expect(response.headers.get("location")).toBe("http://localhost:3000/uz-Latn/dashboard");
  });

  it("does not touch the public landing page regardless of session state", () => {
    const withSession = middleware(makeRequest("/uz-Latn", `${AUTH_COOKIE}=some-token`));
    const withoutSession = middleware(makeRequest("/uz-Latn"));
    expect(withSession.headers.get("location")).toBeNull();
    expect(withoutSession.headers.get("location")).toBeNull();
  });

  it("delegates to next-intl untouched when the URL has no locale prefix yet", () => {
    const response = middleware(makeRequest("/dashboard"));
    expect(response.headers.get("location")).toBeNull(); // the mocked intl middleware returns NextResponse.next()
  });
});
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd "/home/sher/Рабочий стол/avtotest/frontend" && npm test -- middleware`
Expected: FAIL — current `middleware.ts` has no default-exported function named for direct import this way (it exports `createMiddleware(...)`'s result directly as default, which the test's mock replaces with a no-op — so this actually needs the file rewritten, not just failing on missing auth logic). Confirm failure is about missing redirect behavior (test 1 expects a 307, gets whatever the mocked pass-through returns).

- [ ] **Step 3: Rewrite middleware.ts with the auth-guard layer**

`frontend/src/middleware.ts`:

```ts
import createMiddleware from "next-intl/middleware";
import { NextRequest, NextResponse } from "next/server";
import { locales, defaultLocale, type Locale } from "@/i18n/config";
import { AUTH_COOKIE, REFRESH_COOKIE } from "@/lib/auth-cookies";

const intlMiddleware = createMiddleware({ locales, defaultLocale, localePrefix: "always" });

const PROTECTED_SEGMENTS = ["dashboard", "exam-mockup"];
const AUTH_SEGMENTS = ["login"];

function matchesAny(pathname: string, segments: string[]): boolean {
  return segments.some((seg) => pathname === `/${seg}` || pathname.startsWith(`/${seg}/`));
}

export default function middleware(request: NextRequest) {
  const segments = request.nextUrl.pathname.split("/").filter(Boolean);
  const hasLocalePrefix = locales.includes(segments[0] as Locale);

  if (!hasLocalePrefix) {
    return intlMiddleware(request);
  }

  const locale = segments[0];
  const pathname = "/" + segments.slice(1).join("/");
  const hasSession = Boolean(request.cookies.get(AUTH_COOKIE) ?? request.cookies.get(REFRESH_COOKIE));

  if (matchesAny(pathname, PROTECTED_SEGMENTS) && !hasSession) {
    return NextResponse.redirect(new URL(`/${locale}/login`, request.url));
  }
  if (matchesAny(pathname, AUTH_SEGMENTS) && hasSession) {
    return NextResponse.redirect(new URL(`/${locale}/dashboard`, request.url));
  }

  return intlMiddleware(request);
}

export const config = {
  matcher: ["/((?!api|_next|.*\\..*).*)"],
};
```

- [ ] **Step 4: Run to verify it passes, then full suite + build**

```bash
npm test -- middleware && npm run typecheck && npm test && npm run build
```

Expected: all clean.

- [ ] **Step 5: Commit**

```bash
cd "/home/sher/Рабочий стол/avtotest" && git add frontend/ && git commit -m "feat(frontend): auth-guard layer in middleware (redirect protected/login routes)"
```

---

### Task 7: Login UI (phone entry)

**Files:**
- Create: `frontend/src/app/[locale]/(auth)/login/page.tsx`
- Create: `frontend/src/app/[locale]/(auth)/login/page.test.tsx`
- Modify: `frontend/messages/uz-Latn.json`, `frontend/messages/uz-Cyrl.json`, `frontend/messages/ru.json` (add a new top-level `"Login"` key to each)

**Interfaces:**
- Consumes: `Button`/`Card` (Phase A), calls `POST /api/auth/otp/request` (Task 3) by URL.
- Produces: `/login` page navigating to `/login/verify?phone=<digits>` on success.

- [ ] **Step 1: Add the `Login` message key to all three locale files**

Add this object as a new top-level key (sibling to `"ThemeToggle"`, `"Landing"`, etc.) in `frontend/messages/uz-Latn.json`:

```json
"Login": {
  "subtitle": "Telefon raqamingiz bilan kiring",
  "phoneLabel": "Telefon raqam",
  "continue": "Davom etish",
  "errorInvalidPhone": "Telefon raqam noto'g'ri formatda",
  "errorRateLimited": "Juda ko'p urinish. Birozdan keyin qayta urinib ko'ring",
  "errorNetwork": "Server bilan bog'lanib bo'lmadi",
  "errorUnknown": "Xatolik yuz berdi. Qayta urinib ko'ring"
}
```

In `frontend/messages/uz-Cyrl.json`:

```json
"Login": {
  "subtitle": "Телефон рақамингиз билан киринг",
  "phoneLabel": "Телефон рақам",
  "continue": "Давом этиш",
  "errorInvalidPhone": "Телефон рақам нотўғри форматда",
  "errorRateLimited": "Жуда кўп уриниш. Бироздан кейин қайта уриниб кўринг",
  "errorNetwork": "Сервер билан боғланиб бўлмади",
  "errorUnknown": "Хатолик юз берди. Қайта уриниб кўринг"
}
```

In `frontend/messages/ru.json`:

```json
"Login": {
  "subtitle": "Войдите с помощью номера телефона",
  "phoneLabel": "Номер телефона",
  "continue": "Продолжить",
  "errorInvalidPhone": "Неверный формат номера телефона",
  "errorRateLimited": "Слишком много попыток. Повторите позже",
  "errorNetwork": "Не удалось связаться с сервером",
  "errorUnknown": "Произошла ошибка. Попробуйте снова"
}
```

- [ ] **Step 2: Write the failing LoginPage test**

`frontend/src/app/[locale]/(auth)/login/page.test.tsx`:

```tsx
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { describe, it, expect, vi, afterEach } from "vitest";
import messages from "../../../../../messages/uz-Latn.json";
import LoginPage from "./page";

const pushMock = vi.fn();
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: pushMock }),
}));

afterEach(() => {
  vi.unstubAllGlobals();
  pushMock.mockClear();
});

function renderWithIntl() {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      <LoginPage />
    </NextIntlClientProvider>
  );
}

describe("LoginPage", () => {
  it("disables the continue button until 9 digits are entered", () => {
    renderWithIntl();
    const button = screen.getByRole("button", { name: "Davom etish" });
    expect(button).toBeDisabled();
    fireEvent.change(screen.getByLabelText("Telefon raqam"), { target: { value: "901112233" } });
    expect(button).not.toBeDisabled();
  });

  it("requests an OTP and navigates to /login/verify on success", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(new Response(JSON.stringify({ data: { channel: "sandbox" } }), { status: 200 }))
    );
    renderWithIntl();
    fireEvent.change(screen.getByLabelText("Telefon raqam"), { target: { value: "901112233" } });
    fireEvent.click(screen.getByRole("button", { name: "Davom etish" }));

    await waitFor(() => expect(pushMock).toHaveBeenCalledWith("/login/verify?phone=901112233"));
  });

  it("shows a translated error and does not navigate when the backend returns rate_limited", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(new Response(JSON.stringify({ error: { code: "rate_limited" } }), { status: 429 }))
    );
    renderWithIntl();
    fireEvent.change(screen.getByLabelText("Telefon raqam"), { target: { value: "901112233" } });
    fireEvent.click(screen.getByRole("button", { name: "Davom etish" }));

    await waitFor(() =>
      expect(screen.getByText("Juda ko'p urinish. Birozdan keyin qayta urinib ko'ring")).toBeInTheDocument()
    );
    expect(pushMock).not.toHaveBeenCalled();
  });
});
```

- [ ] **Step 3: Run to verify it fails**

Run: `cd "/home/sher/Рабочий стол/avtotest/frontend" && npm test -- "login/page"`
Expected: FAIL — module not found.

- [ ] **Step 4: Implement LoginPage**

`frontend/src/app/[locale]/(auth)/login/page.tsx`:

```tsx
"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";

const ERROR_MESSAGE_KEYS: Record<string, string> = {
  invalid_phone: "errorInvalidPhone",
  rate_limited: "errorRateLimited",
  network_error: "errorNetwork",
};

function normalizePhone(input: string): string {
  return input.replace(/\D/g, "");
}

export default function LoginPage() {
  const t = useTranslations("Login");
  const router = useRouter();
  const [phone, setPhone] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      const res = await fetch("/api/auth/otp/request", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ phone: normalizePhone(phone), channel: "sandbox" }),
      });
      const json = await res.json();
      if (!res.ok) {
        setError(json.error?.code ?? "unknown");
        return;
      }
      router.push(`/login/verify?phone=${encodeURIComponent(normalizePhone(phone))}`);
    } catch {
      setError("network_error");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <main className="flex min-h-screen items-center justify-center px-4">
      <Card className="w-full max-w-sm p-8">
        <h1 className="font-display text-2xl font-bold">AvtoTest</h1>
        <p className="mt-2 text-sm text-muted-foreground">{t("subtitle")}</p>
        <form onSubmit={handleSubmit} className="mt-6 flex flex-col gap-4">
          <div className="flex items-center gap-2 rounded-md border border-border px-3 py-2">
            <span className="text-muted-foreground">+998</span>
            <input
              type="tel"
              inputMode="numeric"
              value={phone}
              onChange={(e) => setPhone(normalizePhone(e.target.value).slice(0, 9))}
              placeholder="90 123 45 67"
              className="w-full bg-transparent outline-none"
              aria-label={t("phoneLabel")}
            />
          </div>
          {error && <p className="text-sm text-danger">{t(ERROR_MESSAGE_KEYS[error] ?? "errorUnknown")}</p>}
          <Button type="submit" variant="game" size="lg" disabled={phone.length !== 9 || submitting}>
            {t("continue")}
          </Button>
        </form>
      </Card>
    </main>
  );
}
```

- [ ] **Step 5: Run to verify it passes, then full suite**

```bash
npm test -- "login/page" && npm run typecheck && npm test
```

Expected: all clean.

- [ ] **Step 6: Commit**

```bash
cd "/home/sher/Рабочий стол/avtotest" && git add frontend/ && git commit -m "feat(frontend): login page (phone entry, real OTP request)"
```

---

### Task 8: OTP verify UI

**Files:**
- Create: `frontend/src/app/[locale]/(auth)/login/verify/page.tsx`
- Create: `frontend/src/app/[locale]/(auth)/login/verify/page.test.tsx`
- Modify: `frontend/messages/uz-Latn.json`, `frontend/messages/uz-Cyrl.json`, `frontend/messages/ru.json` (add a new top-level `"Verify"` key to each)

**Interfaces:**
- Consumes: `Button`/`Card` (Phase A), calls `POST /api/auth/otp/verify` and `POST /api/auth/otp/request` (resend) (Task 3) by URL.
- Produces: `/login/verify` page navigating to `/dashboard` on success.

- [ ] **Step 1: Add the `Verify` message key to all three locale files**

In `frontend/messages/uz-Latn.json`:

```json
"Verify": {
  "title": "Kodni kiriting",
  "subtitle": "{phone} raqamiga yuborilgan 6 xonali kodni kiriting",
  "digitLabel": "{position}-raqam",
  "resend": "Qayta yuborish",
  "resendIn": "Qayta yuborish ({seconds}s)",
  "errorInvalidCode": "Kod noto'g'ri",
  "errorExpiredCode": "Kod muddati tugagan",
  "errorTooManyAttempts": "Juda ko'p urinish",
  "errorNetwork": "Server bilan bog'lanib bo'lmadi",
  "errorUnknown": "Xatolik yuz berdi"
}
```

In `frontend/messages/uz-Cyrl.json`:

```json
"Verify": {
  "title": "Кодни киритинг",
  "subtitle": "{phone} рақамига юборилган 6 хонали кодни киритинг",
  "digitLabel": "{position}-рақам",
  "resend": "Қайта юбориш",
  "resendIn": "Қайта юбориш ({seconds}s)",
  "errorInvalidCode": "Код нотўғри",
  "errorExpiredCode": "Код муддати тугаган",
  "errorTooManyAttempts": "Жуда кўп уриниш",
  "errorNetwork": "Сервер билан боғланиб бўлмади",
  "errorUnknown": "Хатолик юз берди"
}
```

In `frontend/messages/ru.json`:

```json
"Verify": {
  "title": "Введите код",
  "subtitle": "Введите 6-значный код, отправленный на {phone}",
  "digitLabel": "{position}-я цифра",
  "resend": "Отправить снова",
  "resendIn": "Отправить снова ({seconds}с)",
  "errorInvalidCode": "Неверный код",
  "errorExpiredCode": "Код истёк",
  "errorTooManyAttempts": "Слишком много попыток",
  "errorNetwork": "Не удалось связаться с сервером",
  "errorUnknown": "Произошла ошибка"
}
```

- [ ] **Step 2: Write the failing VerifyPage test**

`frontend/src/app/[locale]/(auth)/login/verify/page.test.tsx`:

```tsx
import { render, screen, fireEvent, waitFor, act } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { describe, it, expect, vi, afterEach } from "vitest";
import messages from "../../../../../../messages/uz-Latn.json";
import VerifyPage from "./page";

const pushMock = vi.fn();
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: pushMock }),
  useSearchParams: () => new URLSearchParams("phone=901112233"),
}));

afterEach(() => {
  vi.unstubAllGlobals();
  pushMock.mockClear();
  vi.useRealTimers();
});

function renderWithIntl() {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      <VerifyPage />
    </NextIntlClientProvider>
  );
}

function digitBoxes() {
  return screen.getAllByRole("textbox");
}

describe("VerifyPage", () => {
  it("auto-advances focus to the next digit box as each digit is typed", () => {
    renderWithIntl();
    const boxes = digitBoxes();
    fireEvent.change(boxes[0], { target: { value: "1" } });
    expect(boxes[1]).toHaveFocus();
  });

  it("auto-submits and navigates to /dashboard once all 6 digits are entered correctly", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(new Response(JSON.stringify({ data: { ok: true } }), { status: 200 }))
    );
    renderWithIntl();
    const boxes = digitBoxes();
    "123456".split("").forEach((digit, i) => fireEvent.change(boxes[i], { target: { value: digit } }));

    await waitFor(() => expect(pushMock).toHaveBeenCalledWith("/dashboard"));
  });

  it("shows a translated error and does not navigate on an invalid code", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(new Response(JSON.stringify({ error: { code: "invalid_code" } }), { status: 400 }))
    );
    renderWithIntl();
    const boxes = digitBoxes();
    "000000".split("").forEach((digit, i) => fireEvent.change(boxes[i], { target: { value: digit } }));

    await waitFor(() => expect(screen.getByText("Kod noto'g'ri")).toBeInTheDocument());
    expect(pushMock).not.toHaveBeenCalled();
  });

  it("disables resend during the 60s cooldown and re-enables after it elapses", () => {
    vi.useFakeTimers();
    renderWithIntl();
    const resendButton = screen.getByRole("button");
    expect(resendButton).toBeDisabled();
    act(() => {
      vi.advanceTimersByTime(60_000);
    });
    expect(resendButton).not.toBeDisabled();
  });
});
```

- [ ] **Step 3: Run to verify it fails**

Run: `cd "/home/sher/Рабочий стол/avtotest/frontend" && npm test -- "verify/page"`
Expected: FAIL — module not found.

- [ ] **Step 4: Implement VerifyPage**

`frontend/src/app/[locale]/(auth)/login/verify/page.tsx`:

Next.js requires `useSearchParams()` to sit inside a `<Suspense>` boundary or
`next build` fails with "should be wrapped in a suspense boundary" — split
into an inner form component and a thin default-exported wrapper:

```tsx
"use client";

import { Suspense, useEffect, useRef, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";

const CODE_LENGTH = 6;
const RESEND_COOLDOWN_SECONDS = 60;

const ERROR_MESSAGE_KEYS: Record<string, string> = {
  invalid_code: "errorInvalidCode",
  expired_code: "errorExpiredCode",
  too_many_attempts: "errorTooManyAttempts",
  network_error: "errorNetwork",
};

function VerifyForm() {
  const t = useTranslations("Verify");
  const router = useRouter();
  const searchParams = useSearchParams();
  const phone = searchParams.get("phone") ?? "";

  const [digits, setDigits] = useState<string[]>(Array(CODE_LENGTH).fill(""));
  const [error, setError] = useState<string | null>(null);
  const [cooldown, setCooldown] = useState(RESEND_COOLDOWN_SECONDS);
  const inputRefs = useRef<(HTMLInputElement | null)[]>([]);

  useEffect(() => {
    if (cooldown <= 0) return;
    const timer = setInterval(() => setCooldown((c) => Math.max(0, c - 1)), 1000);
    return () => clearInterval(timer);
  }, [cooldown]);

  async function submitCode(code: string) {
    setError(null);
    try {
      const res = await fetch("/api/auth/otp/verify", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ phone, code }),
      });
      const json = await res.json();
      if (!res.ok) {
        setError(json.error?.code ?? "unknown");
        return;
      }
      router.push("/dashboard");
    } catch {
      setError("network_error");
    }
  }

  function handleDigitChange(index: number, value: string) {
    const clean = value.replace(/\D/g, "").slice(-1);
    const next = [...digits];
    next[index] = clean;
    setDigits(next);

    if (clean && index < CODE_LENGTH - 1) {
      inputRefs.current[index + 1]?.focus();
    }
    if (next.every((d) => d !== "")) {
      void submitCode(next.join(""));
    }
  }

  async function handleResend() {
    setCooldown(RESEND_COOLDOWN_SECONDS);
    await fetch("/api/auth/otp/request", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ phone, channel: "sandbox" }),
    });
  }

  return (
    <main className="flex min-h-screen items-center justify-center px-4">
      <Card className="w-full max-w-sm p-8 text-center">
        <h1 className="font-display text-2xl font-bold">{t("title")}</h1>
        <p className="mt-2 text-sm text-muted-foreground">{t("subtitle", { phone })}</p>
        <div className="mt-6 flex justify-center gap-2">
          {digits.map((d, i) => (
            <input
              key={i}
              ref={(el) => {
                inputRefs.current[i] = el;
              }}
              type="text"
              inputMode="numeric"
              value={d}
              onChange={(e) => handleDigitChange(i, e.target.value)}
              aria-label={t("digitLabel", { position: i + 1 })}
              className="h-12 w-10 rounded-md border border-border bg-card text-center text-lg font-bold outline-none focus:border-accent"
            />
          ))}
        </div>
        {error && <p className="mt-4 text-sm text-danger">{t(ERROR_MESSAGE_KEYS[error] ?? "errorUnknown")}</p>}
        <Button type="button" variant="outline" className="mt-6" disabled={cooldown > 0} onClick={handleResend}>
          {cooldown > 0 ? t("resendIn", { seconds: cooldown }) : t("resend")}
        </Button>
      </Card>
    </main>
  );
}

export default function VerifyPage() {
  return (
    <Suspense fallback={null}>
      <VerifyForm />
    </Suspense>
  );
}
```

- [ ] **Step 5: Run to verify it passes, then full suite + build**

```bash
npm test -- "verify/page" && npm run typecheck && npm test && npm run build
```

Expected: all clean.

- [ ] **Step 6: Commit**

```bash
cd "/home/sher/Рабочий стол/avtotest" && git add frontend/ && git commit -m "feat(frontend): OTP verify page (6-digit auto-advance, resend cooldown)"
```

---

### Task 9: Final verification + live backend smoke test + docs

**Files:**
- Modify: `frontend/README.md`
- Modify: `/home/sher/Рабочий стол/avtotest/README.md` (Frontend section)
- Create: `frontend/.env.local.example`

**Interfaces:** none — this task only verifies and documents Tasks 1-8.

- [ ] **Step 1: Add `.env.local.example`**

`frontend/.env.local.example`:

```
BACKEND_URL=http://localhost:8090
```

- [ ] **Step 2: Update `frontend/README.md`**

Replace its content with:

```markdown
# AvtoTest Frontend

Next.js 14 (App Router) + TypeScript + Tailwind CSS + next-intl.

## Dev

Copy `.env.local.example` to `.env.local` (adjust `BACKEND_URL` if the Go API
runs on a different port). Then:

\`\`\`bash
npm install
npm run dev        # http://localhost:3000 (redirects to /uz-Latn/)
npm run lint
npm run typecheck
npm test
npm run build
\`\`\`

The Go backend must be running for real login/OTP to work: from the repo
root, `make up && make seed && cd backend && PORT=8090 go run ./cmd/api`.

## Phase A mockup routes (still present, still mock-data-driven)

- `/[locale]/` — Landing, `/[locale]/dashboard`, `/[locale]/exam-mockup`

## Phase B1 additions (this phase)

- Real phone+OTP login: `/[locale]/login` → `/[locale]/login/verify` → `/[locale]/dashboard`
- Auth session lives in httpOnly cookies (`at`/`rt`) set by Next.js Route
  Handlers under `/api/auth/*` — no token is ever visible to client JS.
- `/api/proxy/[...path]` is the one path all future authenticated API calls
  go through; it single-flight-refreshes on a 401 and retries once.
- 3 locales (uz-Latn default, uz-Cyrl, ru) via next-intl; middleware redirects
  bare URLs to the default locale and gates `/dashboard`+`/exam-mockup` behind
  a session-cookie check.
- Dashboard/exam-mockup still render `lib/mock-data.ts` after login — wiring
  them to the real backend is Phase B2.
```

- [ ] **Step 3: Update the root README's Frontend section**

Append to `/home/sher/Рабочий стол/avtotest/README.md`'s existing "## Frontend" section (after the Phase A paragraph added previously):

```markdown

**Phase B1 (2026-07-22):** real phone+OTP login, httpOnly-cookie sessions,
single-flight-refresh BFF proxy (`/api/proxy/[...path]`), and full next-intl
locale routing (uz-Latn/uz-Cyrl/ru) landed. Dashboard/exam-mockup content is
still mock data post-login — Phase B2 wires the real backend. Details:
`docs/superpowers/specs/2026-07-22-nextjs-frontend-phase-b1-auth-i18n-design.md`.
```

- [ ] **Step 4: Run the complete verification suite**

```bash
cd "/home/sher/Рабочий стол/avtotest/frontend" && npm run lint && npm run typecheck && npm test && npm run build
```

Expected: every command exits 0. Build's route table should show `/[locale]`, `/[locale]/dashboard`, `/[locale]/exam-mockup`, `/[locale]/login`, `/[locale]/login/verify` plus the `/api/*` routes as dynamic (ƒ) — no page from Tasks 1-8 missing.

- [ ] **Step 5: Live smoke test against the real backend (do this yourself, report the outcome — do not skip)**

```bash
cd "/home/sher/Рабочий стол/avtotest" && make up
cd backend && PORT=8090 go run ./cmd/api &
```

Wait for the API to be ready (`curl localhost:8090/healthz`), then:

```bash
cd "/home/sher/Рабочий стол/avtotest/frontend" && cp .env.local.example .env.local && npm run dev
```

In a browser (or via `curl -c cookies.txt -b cookies.txt` if no browser tool is available — state clearly which method you used): visit `http://localhost:3000/uz-Latn/dashboard` and confirm it redirects to `/uz-Latn/login` (no session yet). Submit a real phone number on `/login`; confirm the sandbox OTP channel returns a `debug_code` (visible in the Go API's response/logs, since `OTP_CHANNEL` defaults to `sandbox`). Enter that code on `/login/verify`; confirm it navigates to `/dashboard` and that a `Set-Cookie` for `at`/`rt` appears in the network response (httpOnly, so not readable from JS — confirm via the Network tab's response headers or curl's cookie jar, not via `document.cookie`). Reload `/uz-Latn/dashboard` — confirm it no longer redirects to login (session persists). Stop both the dev server and the Go API when done.

If you have no browser tool, do the equivalent with curl (`-c`/`-b cookies.txt` to persist cookies across requests) and state explicitly in your report which steps were curl-verified vs. still needing a human's eyes.

- [ ] **Step 6: Commit**

```bash
cd "/home/sher/Рабочий стол/avtotest" && git add frontend/README.md frontend/.env.local.example README.md && git commit -m "docs(frontend): Phase B1 README + env example + live-smoke verification"
```

---

## Self-Review Notes (for whoever executes this plan)

- Every Route Handler is unit-testable by direct import + mocked global `fetch` — no running server needed for Tasks 3-6's tests.
- `readCookie` (Task 2) deliberately avoids `next/headers`'s `cookies()` to stay testable outside Next's request-scoped AsyncLocalStorage context; this is equally correct in production since Route Handlers always receive the real `Cookie` header.
- `callBackendRefresh` (Task 4) is a single shared function reused by both the `/api/auth/refresh` route AND the proxy route's 401-retry path (Task 5) — no duplicated backend-call logic.
- No Zustand anywhere in this plan (approved spec decision — first real use is Phase B3).
- Dashboard/exam-mockup are NOT wired to real data in this plan — they still import `lib/mock-data.ts` after Task 1's relocation. This is intentional; Phase B2 replaces the mock imports with real TanStack Query hooks against `/api/proxy/*`.
