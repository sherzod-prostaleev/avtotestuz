# Next.js Frontend Foundation — Phase A Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stand up the Next.js project with the AvtoTest design system implemented in code, and ship 3 visually-approvable mockup pages (Landing, Dashboard, Exam screen) driven entirely by static mock data — no real API calls, no auth, no i18n runtime yet.

**Architecture:** A single Next.js 14 App Router project at `frontend/` (sibling to `backend/`). Design tokens live as CSS custom properties consumed by a Tailwind color/radius/font extension. A small set of presentational shared components (QuestionCard, AnswerOption, CountdownTimer, MasteryBar, ResultRing) are built test-first and then assembled into the 3 mockup pages using a static `lib/mock-data.ts` module. TanStack Query and Zustand are installed (per the approved spec) but not wired to anything yet — that begins in Phase B.

**Tech Stack:** Next.js 14 (App Router) + React 18 + TypeScript, Tailwind CSS 3, shadcn/ui-style components (hand-rolled `cn` + `cva`, no CLI), next-themes, lucide-react, Vitest + Testing Library.

## Global Constraints

- Repo path is `/home/sher/Рабочий стол/avtotest` — Cyrillic + a space. Always double-quote it in shell commands.
- Project root: `frontend/` (approved 2026-07-21 spec decision).
- **Bosqich A makes ZERO real network calls.** No `fetch` to the Go backend, no `/api/demo/*` calls, no auth, no i18n runtime (`next-intl`), no route stubs for pages this plan doesn't build (`login`, `variants`, `practice`, etc. do not exist yet — creating empty stubs would violate the master prompt's "halol placeholder" rule since nothing links to them yet).
- Design tokens (exact HSL values, derived from the brand hex values fixed in `AVTOTEST-MASTER-PROMPT.txt` §6.0 — `#0E1526` dark bg, `#6C63FF` accent): `--background: 222 46% 10%` (dark) / `228 40% 98%` (light); `--accent: 243 100% 69%` (both themes); semantic colors `--success: 142 71% 45%`, `--danger: 0 84% 60%`, `--streak: 21 90% 54%`, `--gold: 45 93% 47%` (both themes, per spec: "semantik ranglar qat'iy alohida" from brand color).
- Fonts: Baloo 2 (headings/numbers, `font-display` Tailwind utility) + Manrope (body, `font-sans`), both via `next/font/google` (self-hosted, no external CDN request at runtime).
- Buttons: pill shape (`rounded-full`) + bottom 3D press-shadow that disappears and the button shifts down on `:active` (Duolingo-style, spec §6.0).
- State libraries: `@tanstack/react-query` and `zustand` are installed in `package.json` in this plan but consumed by NOTHING until Phase B (explicit approved spec decision — do not wire them prematurely).
- `framer-motion` is explicitly OUT of scope for Phase A (deferred to Phase B) — Phase A validates static visual design only, no animation library needed yet.
- The exam-screen mockup lives at `(app)/exam-mockup/page.tsx`, NOT at the future real `exam/[sessionId]/page.tsx` path — deliberate refinement at plan-time to avoid introducing an unused dynamic route segment for a static mockup; Phase B creates the real dynamic route from scratch.
- Anti-cheat principle carries into the mockup: `AnswerOption` has no `isCorrect` prop and no way to reveal correctness in its `hidden`/`selected`/`neutral` states — callers can only make it show correct/incorrect by explicitly passing `state="correct"` / `state="incorrect"`, mirroring the real backend's behavior of withholding correctness entirely during an exam session.
- Acceptance for every task: `npm run lint`, `npm run typecheck`, `npm test` all clean; no stray console warnings in test output.

---

### Task 1: Project scaffold + tooling

**Files:**
- Create: `frontend/package.json`
- Create: `frontend/tsconfig.json`
- Create: `frontend/next.config.mjs`
- Create: `frontend/postcss.config.js`
- Create: `frontend/tailwind.config.ts`
- Create: `frontend/.eslintrc.json`
- Create: `frontend/.gitignore`
- Create: `frontend/next-env.d.ts`
- Create: `frontend/vitest.config.ts`
- Create: `frontend/vitest.setup.ts`
- Create: `frontend/tests/unit/sanity.test.ts`
- Create: `frontend/src/app/layout.tsx`
- Create: `frontend/src/app/globals.css`
- Create: `frontend/src/app/(public)/page.tsx`

**Interfaces:**
- Produces: a booting Next.js project (`npm run dev`/`npm run build` work), Vitest wired with jsdom + Testing Library, path alias `@/*` → `./src/*`. Every later task builds inside `frontend/src/`.

- [ ] **Step 1: Create the directory and `package.json`**

```bash
mkdir -p "/home/sher/Рабочий стол/avtotest/frontend"
```

`frontend/package.json`:

```json
{
  "name": "avtotest-frontend",
  "version": "0.1.0",
  "private": true,
  "engines": {
    "node": ">=18.17.0"
  },
  "scripts": {
    "dev": "next dev",
    "build": "next build",
    "start": "next start",
    "lint": "next lint",
    "typecheck": "tsc --noEmit",
    "test": "vitest run"
  },
  "dependencies": {
    "next": "^14.2.0",
    "react": "^18.3.0",
    "react-dom": "^18.3.0",
    "next-themes": "^0.3.0",
    "@tanstack/react-query": "^5.51.0",
    "zustand": "^4.5.0",
    "class-variance-authority": "^0.7.0",
    "clsx": "^2.1.0",
    "tailwind-merge": "^2.5.0",
    "lucide-react": "^0.427.0",
    "@radix-ui/react-slot": "^1.1.0"
  },
  "devDependencies": {
    "typescript": "^5.5.0",
    "@types/node": "^20.14.0",
    "@types/react": "^18.3.0",
    "@types/react-dom": "^18.3.0",
    "tailwindcss": "^3.4.0",
    "postcss": "^8.4.0",
    "autoprefixer": "^10.4.0",
    "eslint": "^8.57.0",
    "eslint-config-next": "^14.2.0",
    "vitest": "^2.0.0",
    "@vitejs/plugin-react": "^4.3.0",
    "jsdom": "^24.1.0",
    "@testing-library/react": "^16.0.0",
    "@testing-library/jest-dom": "^6.4.0"
  }
}
```

- [ ] **Step 2: Create TypeScript, Next, PostCSS, ESLint, gitignore config**

`frontend/tsconfig.json`:

```json
{
  "compilerOptions": {
    "target": "ES2017",
    "lib": ["dom", "dom.iterable", "esnext"],
    "allowJs": true,
    "skipLibCheck": true,
    "strict": true,
    "noEmit": true,
    "esModuleInterop": true,
    "module": "esnext",
    "moduleResolution": "bundler",
    "resolveJsonModule": true,
    "isolatedModules": true,
    "jsx": "preserve",
    "incremental": true,
    "plugins": [{ "name": "next" }],
    "baseUrl": ".",
    "paths": {
      "@/*": ["./src/*"]
    }
  },
  "include": ["next-env.d.ts", "**/*.ts", "**/*.tsx", ".next/types/**/*.ts"],
  "exclude": ["node_modules"]
}
```

`frontend/next-env.d.ts`:

```ts
/// <reference types="next" />
/// <reference types="next/image-types/global" />

// NOTE: This file should not be edited
// see https://nextjs.org/docs/app/building-your-application/configuring/typescript for more information.
```

`frontend/next.config.mjs`:

```js
/** @type {import('next').NextConfig} */
const nextConfig = {};

export default nextConfig;
```

`frontend/postcss.config.js`:

```js
module.exports = {
  plugins: {
    tailwindcss: {},
    autoprefixer: {},
  },
};
```

`frontend/tailwind.config.ts` (minimal for now — Task 2 replaces this with the full token mapping):

```ts
import type { Config } from "tailwindcss";

const config: Config = {
  darkMode: ["class"],
  content: ["./src/**/*.{ts,tsx}"],
  theme: {
    extend: {},
  },
  plugins: [],
};
export default config;
```

`frontend/.eslintrc.json`:

```json
{
  "extends": "next/core-web-vitals"
}
```

`frontend/.gitignore`:

```
node_modules
.next
out
build
*.tsbuildinfo
.env*.local
coverage
```

- [ ] **Step 3: Create Vitest config + setup + sanity test**

`frontend/vitest.config.ts`:

```ts
import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";
import path from "path";

export default defineConfig({
  plugins: [react()],
  test: {
    environment: "jsdom",
    setupFiles: ["./vitest.setup.ts"],
  },
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
});
```

`frontend/vitest.setup.ts`:

```ts
import "@testing-library/jest-dom/vitest";

// next-themes (and other libraries) call matchMedia even when enableSystem
// is false; jsdom doesn't implement it, so every test using ThemeProvider
// would throw without this shim.
if (!window.matchMedia) {
  window.matchMedia = ((query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: () => {},
    removeListener: () => {},
    addEventListener: () => {},
    removeEventListener: () => {},
    dispatchEvent: () => false,
  })) as unknown as typeof window.matchMedia;
}
```

`frontend/tests/unit/sanity.test.ts`:

```ts
import { describe, it, expect } from "vitest";

describe("sanity", () => {
  it("runs", () => {
    expect(1 + 1).toBe(2);
  });
});
```

- [ ] **Step 4: Create the root layout, global CSS, and a placeholder landing page**

`frontend/src/app/globals.css`:

```css
@tailwind base;
@tailwind components;
@tailwind utilities;
```

`frontend/src/app/layout.tsx`:

```tsx
import "./globals.css";

export const metadata = {
  title: "AvtoTest",
  description: "Haydovchilik nazariy imtihoniga tayyorgarlik",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="uz">
      <body>{children}</body>
    </html>
  );
}
```

`frontend/src/app/(public)/page.tsx`:

```tsx
export default function LandingPage() {
  return <main className="p-8">AvtoTest — tez orada.</main>;
}
```

- [ ] **Step 5: Install dependencies and verify**

```bash
cd "/home/sher/Рабочий стол/avtotest/frontend" && npm install
```

Expected: installs without error (a peer-dependency warning about React 18 vs some package's optional React 19 peer range is acceptable noise; an actual ERESOLVE failure is not — if one occurs, report BLOCKED with the exact error rather than adding `--legacy-peer-deps` silently).

```bash
npm run typecheck
```

Expected: no output, exit 0.

```bash
npm test
```

Expected: `sanity.test.ts` — 1 passed.

```bash
npm run lint
```

Expected: "No ESLint warnings or errors" (or equivalent clean output).

```bash
npm run build
```

Expected: build succeeds, prints the route table including `/`.

- [ ] **Step 6: Commit**

```bash
cd "/home/sher/Рабочий стол/avtotest" && git add frontend/ && git commit -m "feat(frontend): scaffold Next.js 14 project (TypeScript, Tailwind, Vitest)"
```

---

### Task 2: Design tokens + theming

**Files:**
- Modify: `frontend/src/app/globals.css`
- Modify: `frontend/tailwind.config.ts`
- Modify: `frontend/src/app/layout.tsx`
- Create: `frontend/src/app/providers.tsx`
- Create: `frontend/src/components/theme-toggle.tsx`
- Create: `frontend/src/components/theme-toggle.test.tsx`

**Interfaces:**
- Consumes: nothing from Task 1 beyond the booting project.
- Produces: CSS custom properties (`--background`, `--foreground`, `--card`, `--card-foreground`, `--border`, `--input`, `--muted-foreground`, `--accent`, `--accent-foreground`, `--accent-shadow`, `--success`, `--danger`, `--streak`, `--gold`, `--ring`, `--radius`) and their Tailwind color-name mappings (`bg-background`, `text-foreground`, `bg-card`, `border-border`, `text-accent`, `bg-accent`, `text-success`, `text-danger`, `text-streak`, `text-gold`, plus `font-display`/`font-sans` utilities) — every later task's className strings assume these exist. `<ThemeToggle />` component for later pages to place in a header.

- [ ] **Step 1: Write the failing ThemeToggle test**

`frontend/src/components/theme-toggle.test.tsx`:

```tsx
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { ThemeProvider } from "next-themes";
import { ThemeToggle } from "./theme-toggle";

function renderWithTheme() {
  return render(
    <ThemeProvider attribute="class" defaultTheme="dark" enableSystem={false}>
      <ThemeToggle />
    </ThemeProvider>
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

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd "/home/sher/Рабочий стол/avtotest/frontend" && npm test -- theme-toggle`
Expected: FAIL — `Failed to resolve import "./theme-toggle"` (file doesn't exist yet).

- [ ] **Step 3: Write the full design tokens (globals.css)**

`frontend/src/app/globals.css`:

```css
@tailwind base;
@tailwind components;
@tailwind utilities;

:root {
  --background: 228 40% 98%;
  --foreground: 222 47% 11%;
  --card: 0 0% 100%;
  --card-foreground: 222 47% 11%;
  --border: 220 20% 90%;
  --input: 220 20% 90%;
  --muted-foreground: 220 15% 40%;
  --accent: 243 100% 69%;
  --accent-foreground: 0 0% 100%;
  --accent-shadow: 243 60% 45%;
  --success: 142 71% 45%;
  --danger: 0 84% 60%;
  --streak: 21 90% 54%;
  --gold: 45 93% 47%;
  --ring: 243 100% 69%;
  --radius: 1.25rem;
}

.dark {
  --background: 222 46% 10%;
  --foreground: 210 40% 98%;
  --card: 225 41% 18%;
  --card-foreground: 210 40% 98%;
  --border: 225 30% 27%;
  --input: 225 30% 27%;
  --muted-foreground: 217 20% 65%;
  --accent: 243 100% 69%;
  --accent-foreground: 0 0% 100%;
  --accent-shadow: 243 60% 45%;
  --success: 142 71% 45%;
  --danger: 0 84% 60%;
  --streak: 21 90% 54%;
  --gold: 45 93% 47%;
  --ring: 243 100% 69%;
}

body {
  @apply bg-background text-foreground font-sans;
}
```

- [ ] **Step 4: Wire the tokens into Tailwind + fonts**

`frontend/tailwind.config.ts`:

```ts
import type { Config } from "tailwindcss";

const config: Config = {
  darkMode: ["class"],
  content: ["./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        background: "hsl(var(--background))",
        foreground: "hsl(var(--foreground))",
        card: {
          DEFAULT: "hsl(var(--card))",
          foreground: "hsl(var(--card-foreground))",
        },
        border: "hsl(var(--border))",
        input: "hsl(var(--input))",
        ring: "hsl(var(--ring))",
        "muted-foreground": "hsl(var(--muted-foreground))",
        accent: {
          DEFAULT: "hsl(var(--accent))",
          foreground: "hsl(var(--accent-foreground))",
        },
        success: "hsl(var(--success))",
        danger: "hsl(var(--danger))",
        streak: "hsl(var(--streak))",
        gold: "hsl(var(--gold))",
      },
      borderRadius: {
        lg: "var(--radius)",
        md: "calc(var(--radius) - 4px)",
        sm: "calc(var(--radius) - 8px)",
      },
      fontFamily: {
        display: ["var(--font-baloo)"],
        sans: ["var(--font-manrope)"],
      },
    },
  },
  plugins: [],
};
export default config;
```

- [ ] **Step 5: Wire fonts + ThemeProvider into the root layout**

`frontend/src/app/providers.tsx`:

```tsx
"use client";

import { ThemeProvider } from "next-themes";

export function Providers({ children }: { children: React.ReactNode }) {
  return (
    <ThemeProvider attribute="class" defaultTheme="dark" enableSystem={false}>
      {children}
    </ThemeProvider>
  );
}
```

`frontend/src/app/layout.tsx`:

```tsx
import type { Metadata } from "next";
import { Baloo_2, Manrope } from "next/font/google";
import { Providers } from "./providers";
import "./globals.css";

const baloo = Baloo_2({ subsets: ["latin"], weight: ["600", "700", "800"], variable: "--font-baloo" });
const manrope = Manrope({ subsets: ["latin"], weight: ["400", "500", "600", "700"], variable: "--font-manrope" });

export const metadata: Metadata = {
  title: "AvtoTest",
  description: "Haydovchilik nazariy imtihoniga tayyorgarlik",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="uz" suppressHydrationWarning className={`${baloo.variable} ${manrope.variable}`}>
      <body>
        <Providers>{children}</Providers>
      </body>
    </html>
  );
}
```

- [ ] **Step 6: Implement ThemeToggle**

`frontend/src/components/theme-toggle.tsx`:

```tsx
"use client";

import { useEffect, useState } from "react";
import { useTheme } from "next-themes";
import { Moon, Sun } from "lucide-react";

export function ThemeToggle() {
  const { theme, setTheme } = useTheme();
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
      aria-label={isDark ? "Yorug' temaga o'tish" : "Qorong'i temaga o'tish"}
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

- [ ] **Step 7: Run the test to verify it passes**

Run: `npm test -- theme-toggle`
Expected: 2 passed.

- [ ] **Step 8: Run the full suite + build**

```bash
npm run typecheck && npm test && npm run build
```

Expected: all clean; build succeeds (the placeholder landing page now renders with real tokens/fonts applied, though nothing visually elaborate yet).

- [ ] **Step 9: Commit**

```bash
cd "/home/sher/Рабочий стол/avtotest" && git add frontend/ && git commit -m "feat(frontend): design tokens (colors, fonts, dark/light theme)"
```

---

### Task 3: shadcn-style Button + Card

**Files:**
- Create: `frontend/src/lib/utils.ts`
- Create: `frontend/src/components/ui/button.tsx`
- Create: `frontend/src/components/ui/button.test.tsx`
- Create: `frontend/src/components/ui/card.tsx`

**Interfaces:**
- Consumes: `--accent`, `--accent-shadow`, `--card`, `--border` tokens and their Tailwind mappings (Task 2).
- Produces: `Button({ variant?: "default"|"game"|"outline", size?: "default"|"sm"|"lg", asChild?: boolean, ...ButtonHTMLAttributes })` and `Card` (a plain `div` wrapper) — every later shared component and page imports these from `@/components/ui/button` and `@/components/ui/card`.

- [ ] **Step 1: Write the failing Button test**

`frontend/src/components/ui/button.test.tsx`:

```tsx
import { render } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { Button } from "./button";

describe("Button", () => {
  it("applies pill shape and 3D press-shadow classes for the game variant", () => {
    const { getByRole } = render(<Button variant="game">Boshlash</Button>);
    const button = getByRole("button", { name: "Boshlash" });
    expect(button.className).toContain("rounded-full");
    expect(button.className).toContain("active:translate-y-1");
  });

  it("defaults to the standard rounded-md variant when no variant is given", () => {
    const { getByRole } = render(<Button>Davom etish</Button>);
    const button = getByRole("button", { name: "Davom etish" });
    expect(button.className).toContain("rounded-md");
    expect(button.className).not.toContain("rounded-full");
  });
});
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `npm test -- button.test`
Expected: FAIL — `Failed to resolve import "./button"`.

- [ ] **Step 3: Write `lib/utils.ts` and the Button/Card implementations**

`frontend/src/lib/utils.ts`:

```ts
import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}
```

`frontend/src/components/ui/button.tsx`:

```tsx
import * as React from "react";
import { Slot } from "@radix-ui/react-slot";
import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "@/lib/utils";

const buttonVariants = cva(
  "inline-flex items-center justify-center whitespace-nowrap text-sm font-semibold transition-all disabled:pointer-events-none disabled:opacity-50",
  {
    variants: {
      variant: {
        default: "rounded-md bg-accent text-accent-foreground hover:opacity-90",
        game: "rounded-full bg-accent text-accent-foreground shadow-[0_4px_0_0_hsl(var(--accent-shadow))] active:translate-y-1 active:shadow-none",
        outline: "rounded-md border border-border bg-transparent hover:bg-card",
      },
      size: {
        default: "h-10 px-4 py-2",
        lg: "h-14 px-8 text-base",
        sm: "h-8 px-3 text-xs",
      },
    },
    defaultVariants: { variant: "default", size: "default" },
  }
);

export interface ButtonProps
  extends React.ButtonHTMLAttributes<HTMLButtonElement>,
    VariantProps<typeof buttonVariants> {
  asChild?: boolean;
}

const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant, size, asChild = false, ...props }, ref) => {
    const Comp = asChild ? Slot : "button";
    return <Comp className={cn(buttonVariants({ variant, size, className }))} ref={ref} {...props} />;
  }
);
Button.displayName = "Button";

export { Button, buttonVariants };
```

`frontend/src/components/ui/card.tsx`:

```tsx
import * as React from "react";
import { cn } from "@/lib/utils";

const Card = React.forwardRef<HTMLDivElement, React.HTMLAttributes<HTMLDivElement>>(
  ({ className, ...props }, ref) => (
    <div ref={ref} className={cn("rounded-lg border border-border bg-card text-card-foreground", className)} {...props} />
  )
);
Card.displayName = "Card";

export { Card };
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `npm test -- button.test`
Expected: 2 passed.

- [ ] **Step 5: Run the full suite + typecheck**

```bash
npm run typecheck && npm test
```

Expected: clean.

- [ ] **Step 6: Commit**

```bash
cd "/home/sher/Рабочий стол/avtotest" && git add frontend/ && git commit -m "feat(frontend): shadcn-style Button (pill+3D variant) and Card"
```

---

### Task 4: QuestionCard + AnswerOption

**Files:**
- Create: `frontend/src/components/shared/question-card.tsx`
- Create: `frontend/src/components/shared/question-card.test.tsx`
- Create: `frontend/src/components/shared/answer-option.tsx`
- Create: `frontend/src/components/shared/answer-option.test.tsx`

**Interfaces:**
- Consumes: `Card` (Task 3), design tokens (Task 2).
- Produces: `QuestionCard({ questionNumber: number, totalQuestions: number, text: string, hasImage?: boolean })`; `AnswerOption({ shortcutLabel: string, text: string, state: AnswerState, onSelect?: () => void })` where `type AnswerState = "neutral" | "selected" | "correct" | "incorrect" | "hidden"`. Both are pure/presentational — no data fetching. Tasks 7/8/9 import these from `@/components/shared/question-card` and `@/components/shared/answer-option`.

- [ ] **Step 1: Write the failing QuestionCard test**

`frontend/src/components/shared/question-card.test.tsx`:

```tsx
import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { QuestionCard } from "./question-card";

describe("QuestionCard", () => {
  it("renders the question position and text", () => {
    render(<QuestionCard questionNumber={3} totalQuestions={20} text="Chorrahada kim ustunlikka ega?" />);
    expect(screen.getByText("Savol 3 / 20")).toBeInTheDocument();
    expect(screen.getByText("Chorrahada kim ustunlikka ega?")).toBeInTheDocument();
  });

  it("shows an image placeholder only when hasImage is true", () => {
    const { rerender } = render(
      <QuestionCard questionNumber={1} totalQuestions={20} text="Savol matni" hasImage={false} />
    );
    expect(screen.queryByText("Savol rasmi")).not.toBeInTheDocument();
    rerender(<QuestionCard questionNumber={1} totalQuestions={20} text="Savol matni" hasImage />);
    expect(screen.getByText("Savol rasmi")).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd "/home/sher/Рабочий стол/avtotest/frontend" && npm test -- question-card`
Expected: FAIL — module not found.

- [ ] **Step 3: Implement QuestionCard**

`frontend/src/components/shared/question-card.tsx`:

```tsx
import { ImageIcon } from "lucide-react";
import { Card } from "@/components/ui/card";

export interface QuestionCardProps {
  questionNumber: number;
  totalQuestions: number;
  text: string;
  hasImage?: boolean;
}

export function QuestionCard({ questionNumber, totalQuestions, text, hasImage = false }: QuestionCardProps) {
  return (
    <Card className="p-6 md:p-8">
      <p className="mb-4 text-sm font-semibold text-muted-foreground">
        Savol {questionNumber} / {totalQuestions}
      </p>
      <p className="font-display text-xl font-bold leading-snug md:text-2xl">{text}</p>
      {hasImage && (
        <div className="mt-6 flex h-48 items-center justify-center rounded-md border border-dashed border-border bg-background/40 text-muted-foreground">
          <ImageIcon aria-hidden className="mr-2 h-6 w-6" />
          <span className="text-sm">Savol rasmi</span>
        </div>
      )}
    </Card>
  );
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `npm test -- question-card`
Expected: 2 passed.

- [ ] **Step 5: Write the failing AnswerOption test**

`frontend/src/components/shared/answer-option.test.tsx`:

```tsx
import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { AnswerOption } from "./answer-option";

describe("AnswerOption", () => {
  it("shows a checkmark only in the correct state", () => {
    render(<AnswerOption shortcutLabel="F1" text="To'g'ridan burilish" state="correct" />);
    expect(screen.getByTestId("answer-correct-icon")).toBeInTheDocument();
  });

  it("shows an x-mark only in the incorrect state", () => {
    render(<AnswerOption shortcutLabel="F2" text="Chapga burilish" state="incorrect" />);
    expect(screen.getByTestId("answer-incorrect-icon")).toBeInTheDocument();
  });

  it("calls onSelect when clicked", () => {
    const onSelect = vi.fn();
    render(<AnswerOption shortcutLabel="F1" text="Variant" state="neutral" onSelect={onSelect} />);
    fireEvent.click(screen.getByRole("button"));
    expect(onSelect).toHaveBeenCalledOnce();
  });

  it("never exposes a way to read correctness in the hidden (exam) state", () => {
    render(<AnswerOption shortcutLabel="F1" text="Variant" state="hidden" />);
    expect(screen.queryByTestId("answer-correct-icon")).not.toBeInTheDocument();
    expect(screen.queryByTestId("answer-incorrect-icon")).not.toBeInTheDocument();
  });
});
```

- [ ] **Step 6: Run to verify it fails**

Run: `npm test -- answer-option`
Expected: FAIL — module not found.

- [ ] **Step 7: Implement AnswerOption**

`frontend/src/components/shared/answer-option.tsx`:

```tsx
import { Check, X } from "lucide-react";
import { cn } from "@/lib/utils";

export type AnswerState = "neutral" | "selected" | "correct" | "incorrect" | "hidden";

export interface AnswerOptionProps {
  shortcutLabel: string;
  text: string;
  state: AnswerState;
  onSelect?: () => void;
}

const stateClasses: Record<AnswerState, string> = {
  neutral: "border-border bg-card hover:border-accent",
  selected: "border-accent bg-accent/10",
  correct: "border-success bg-success/15",
  incorrect: "border-danger bg-danger/15",
  hidden: "border-border bg-card",
};

// NOTE: there is deliberately no `isCorrect` prop. The caller can only make
// this component reveal correctness by explicitly passing state="correct" /
// "incorrect" — in exam mode the caller only ever has "neutral"/"selected"/
// "hidden" available because the backend withholds correctness entirely
// until the session ends (see backend/internal/session — anti-cheat).
export function AnswerOption({ shortcutLabel, text, state, onSelect }: AnswerOptionProps) {
  return (
    <button
      type="button"
      onClick={onSelect}
      className={cn(
        "flex w-full items-center gap-4 rounded-lg border-2 px-4 py-3 text-left transition-colors",
        stateClasses[state]
      )}
    >
      <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-md border border-border font-display text-sm font-bold">
        {shortcutLabel}
      </span>
      <span className="flex-1 font-medium">{text}</span>
      {state === "correct" && <Check data-testid="answer-correct-icon" aria-hidden className="h-5 w-5 text-success" />}
      {state === "incorrect" && <X data-testid="answer-incorrect-icon" aria-hidden className="h-5 w-5 text-danger" />}
    </button>
  );
}
```

- [ ] **Step 8: Run to verify it passes, then full suite**

```bash
npm test -- answer-option && npm run typecheck && npm test
```

Expected: all clean.

- [ ] **Step 9: Commit**

```bash
cd "/home/sher/Рабочий стол/avtotest" && git add frontend/ && git commit -m "feat(frontend): QuestionCard and AnswerOption shared components"
```

---

### Task 5: CountdownTimer + MasteryBar + ResultRing

**Files:**
- Create: `frontend/src/components/shared/countdown-timer.tsx`
- Create: `frontend/src/components/shared/countdown-timer.test.tsx`
- Create: `frontend/src/components/shared/mastery-bar.tsx`
- Create: `frontend/src/components/shared/mastery-bar.test.tsx`
- Create: `frontend/src/components/shared/result-ring.tsx`
- Create: `frontend/src/components/shared/result-ring.test.tsx`

**Interfaces:**
- Consumes: design tokens (Task 2).
- Produces: `CountdownTimer({ remainingSeconds: number })`; `MasteryBar({ categoryName: string, masteryPercent: number })`; `ResultRing({ percent: number, label?: string })`. All pure/presentational, no timers/intervals (Phase A doesn't tick — callers control `remainingSeconds` directly).

- [ ] **Step 1: Write the failing CountdownTimer test**

`frontend/src/components/shared/countdown-timer.test.tsx`:

```tsx
import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { CountdownTimer } from "./countdown-timer";

describe("CountdownTimer", () => {
  it("formats minutes and seconds with zero-padding", () => {
    render(<CountdownTimer remainingSeconds={125} />);
    expect(screen.getByTestId("countdown-timer")).toHaveTextContent("2:05");
  });

  it("uses the gold color when more than a minute remains", () => {
    render(<CountdownTimer remainingSeconds={90} />);
    expect(screen.getByTestId("countdown-timer").className).toContain("text-gold");
  });

  it("switches to a pulsating red state in the last minute", () => {
    render(<CountdownTimer remainingSeconds={45} />);
    const el = screen.getByTestId("countdown-timer");
    expect(el.className).toContain("text-danger");
    expect(el.className).toContain("animate-pulse");
  });

  it("never shows a negative time", () => {
    render(<CountdownTimer remainingSeconds={-5} />);
    expect(screen.getByTestId("countdown-timer")).toHaveTextContent("0:00");
  });
});
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd "/home/sher/Рабочий стол/avtotest/frontend" && npm test -- countdown-timer`
Expected: FAIL — module not found.

- [ ] **Step 3: Implement CountdownTimer**

`frontend/src/components/shared/countdown-timer.tsx`:

```tsx
import { cn } from "@/lib/utils";

export interface CountdownTimerProps {
  remainingSeconds: number;
}

function formatTime(totalSeconds: number): string {
  const clamped = Math.max(0, totalSeconds);
  const minutes = Math.floor(clamped / 60);
  const seconds = clamped % 60;
  return `${minutes}:${seconds.toString().padStart(2, "0")}`;
}

export function CountdownTimer({ remainingSeconds }: CountdownTimerProps) {
  const isLowTime = remainingSeconds <= 60;
  return (
    <span
      data-testid="countdown-timer"
      className={cn(
        "font-display text-2xl font-bold tabular-nums",
        isLowTime ? "animate-pulse text-danger" : "text-gold"
      )}
    >
      {formatTime(remainingSeconds)}
    </span>
  );
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `npm test -- countdown-timer`
Expected: 4 passed.

- [ ] **Step 5: Write the failing MasteryBar test**

`frontend/src/components/shared/mastery-bar.test.tsx`:

```tsx
import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { MasteryBar } from "./mastery-bar";

describe("MasteryBar", () => {
  it("colors the fill red below 40%", () => {
    render(<MasteryBar categoryName="Chorrahalar" masteryPercent={25} />);
    expect(screen.getByTestId("mastery-bar-fill").className).toContain("bg-danger");
  });

  it("colors the fill gold between 40% and 79%", () => {
    render(<MasteryBar categoryName="Chorrahalar" masteryPercent={55} />);
    expect(screen.getByTestId("mastery-bar-fill").className).toContain("bg-gold");
  });

  it("colors the fill green at 80% and above", () => {
    render(<MasteryBar categoryName="Chorrahalar" masteryPercent={92} />);
    expect(screen.getByTestId("mastery-bar-fill").className).toContain("bg-success");
  });

  it("clamps the rendered width to 0-100", () => {
    render(<MasteryBar categoryName="Chorrahalar" masteryPercent={150} />);
    expect(screen.getByTestId("mastery-bar-fill")).toHaveStyle({ width: "100%" });
  });
});
```

- [ ] **Step 6: Run to verify it fails, then implement**

Run: `npm test -- mastery-bar` → FAIL (module not found).

`frontend/src/components/shared/mastery-bar.tsx`:

```tsx
import { cn } from "@/lib/utils";

export interface MasteryBarProps {
  categoryName: string;
  masteryPercent: number;
}

function colorForMastery(percent: number): string {
  if (percent >= 80) return "bg-success";
  if (percent >= 40) return "bg-gold";
  return "bg-danger";
}

export function MasteryBar({ categoryName, masteryPercent }: MasteryBarProps) {
  const clamped = Math.min(100, Math.max(0, masteryPercent));
  return (
    <div>
      <div className="mb-1 flex items-center justify-between text-sm">
        <span className="font-medium">{categoryName}</span>
        <span className="text-muted-foreground">{clamped}%</span>
      </div>
      <div className="h-2 w-full overflow-hidden rounded-full bg-border" data-testid="mastery-bar-track">
        <div
          data-testid="mastery-bar-fill"
          className={cn("h-full rounded-full transition-all", colorForMastery(clamped))}
          style={{ width: `${clamped}%` }}
        />
      </div>
    </div>
  );
}
```

Run: `npm test -- mastery-bar` → 4 passed.

- [ ] **Step 7: Write the failing ResultRing test**

`frontend/src/components/shared/result-ring.test.tsx`:

```tsx
import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { ResultRing } from "./result-ring";

describe("ResultRing", () => {
  it("renders the clamped percentage as text", () => {
    render(<ResultRing percent={68} label="Tayyorlik" />);
    expect(screen.getByText("68%")).toBeInTheDocument();
    expect(screen.getByText("Tayyorlik")).toBeInTheDocument();
  });

  it("uses the gold ring color at 80% and above", () => {
    render(<ResultRing percent={85} />);
    expect(screen.getByTestId("result-ring-progress").getAttribute("class")).toContain("text-gold");
  });

  it("uses the danger ring color below 50%", () => {
    render(<ResultRing percent={30} />);
    expect(screen.getByTestId("result-ring-progress").getAttribute("class")).toContain("text-danger");
  });

  it("computes a full stroke offset (no progress) at 0%", () => {
    render(<ResultRing percent={0} />);
    const circle = screen.getByTestId("result-ring-progress");
    expect(circle.getAttribute("stroke-dashoffset")).toBe(String(2 * Math.PI * 54));
  });
});
```

- [ ] **Step 8: Run to verify it fails, then implement**

Run: `npm test -- result-ring` → FAIL (module not found).

`frontend/src/components/shared/result-ring.tsx`:

```tsx
import { cn } from "@/lib/utils";

export interface ResultRingProps {
  percent: number;
  label?: string;
}

const RADIUS = 54;
const CIRCUMFERENCE = 2 * Math.PI * RADIUS;

export function ResultRing({ percent, label }: ResultRingProps) {
  const clamped = Math.min(100, Math.max(0, percent));
  const offset = CIRCUMFERENCE * (1 - clamped / 100);
  const ringColorClass = clamped >= 80 ? "text-gold" : clamped >= 50 ? "text-accent" : "text-danger";

  return (
    <div className="relative inline-flex h-32 w-32 items-center justify-center">
      <svg viewBox="0 0 120 120" className="h-full w-full -rotate-90">
        <circle cx="60" cy="60" r={RADIUS} strokeWidth="10" className="fill-none stroke-border" />
        <circle
          data-testid="result-ring-progress"
          cx="60"
          cy="60"
          r={RADIUS}
          strokeWidth="10"
          strokeLinecap="round"
          strokeDasharray={CIRCUMFERENCE}
          strokeDashoffset={offset}
          className={cn("fill-none stroke-current transition-all", ringColorClass)}
        />
      </svg>
      <div className="absolute flex flex-col items-center">
        <span className="font-display text-3xl font-extrabold">{clamped}%</span>
        {label && <span className="text-xs text-muted-foreground">{label}</span>}
      </div>
    </div>
  );
}
```

- [ ] **Step 9: Run to verify it passes, then the full suite**

```bash
npm test -- result-ring && npm run typecheck && npm test
```

Expected: all clean.

- [ ] **Step 10: Commit**

```bash
cd "/home/sher/Рабочий стол/avtotest" && git add frontend/ && git commit -m "feat(frontend): CountdownTimer, MasteryBar, ResultRing shared components"
```

---

### Task 6: Mock data module

**Files:**
- Create: `frontend/src/lib/mock-data.ts`
- Create: `frontend/src/lib/mock-data.test.ts`

**Interfaces:**
- Consumes: nothing (pure data).
- Produces: `demoQuestion: MockQuestion`, `mockExamQuestions: MockQuestion[]`, `mockProfile: { name, isVip, streak: { current, best, todayDone, dailyGoal }, readinessPercent }`, `mockCategoryMastery: { categoryName, masteryPercent }[]`, `proofStats: { value, label }[]`, and the `MockQuestion`/`MockAnswer` types. Tasks 7/8/9 import from `@/lib/mock-data`.

- [ ] **Step 1: Write the failing invariants test**

`frontend/src/lib/mock-data.test.ts`:

```ts
import { describe, it, expect } from "vitest";
import { demoQuestion, mockExamQuestions, mockProfile, mockCategoryMastery, proofStats } from "./mock-data";

describe("mock-data invariants", () => {
  it("every mock question's correctAnswerId matches one of its own answers", () => {
    for (const q of [demoQuestion, ...mockExamQuestions]) {
      const ids = q.answers.map((a) => a.id);
      expect(ids).toContain(q.correctAnswerId);
    }
  });

  it("every mock question has between 2 and 5 answers (matches the real content invariant)", () => {
    for (const q of [demoQuestion, ...mockExamQuestions]) {
      expect(q.answers.length).toBeGreaterThanOrEqual(2);
      expect(q.answers.length).toBeLessThanOrEqual(5);
    }
  });

  it("streak progress never exceeds the daily goal in the mock profile", () => {
    expect(mockProfile.streak.todayDone).toBeLessThanOrEqual(mockProfile.streak.dailyGoal);
  });

  it("category mastery percentages are within 0-100", () => {
    for (const c of mockCategoryMastery) {
      expect(c.masteryPercent).toBeGreaterThanOrEqual(0);
      expect(c.masteryPercent).toBeLessThanOrEqual(100);
    }
  });

  it("proof stats has exactly 4 entries matching the landing page design", () => {
    expect(proofStats).toHaveLength(4);
  });
});
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd "/home/sher/Рабочий стол/avtotest/frontend" && npm test -- mock-data`
Expected: FAIL — module not found.

- [ ] **Step 3: Implement the mock data module**

`frontend/src/lib/mock-data.ts`:

```ts
export interface MockAnswer {
  id: string;
  shortcutLabel: string;
  text: string;
}

export interface MockQuestion {
  id: string;
  text: string;
  hasImage: boolean;
  answers: MockAnswer[];
  correctAnswerId: string;
}

export const demoQuestion: MockQuestion = {
  id: "demo-1",
  text: "Ushbu chorrahada svetofor ishlamayapti. Kim birinchi bo'lib o'tadi?",
  hasImage: true,
  answers: [
    { id: "a1", shortcutLabel: "F1", text: "Chapdan keluvchi haydovchi" },
    { id: "a2", shortcutLabel: "F2", text: "O'ngdan keluvchi haydovchi" },
    { id: "a3", shortcutLabel: "F3", text: "To'g'ri harakatlanuvchi haydovchi" },
    { id: "a4", shortcutLabel: "F4", text: "Kim tezroq yetib borsa" },
  ],
  correctAnswerId: "a2",
};

export const mockExamQuestions: MockQuestion[] = [
  demoQuestion,
  {
    id: "q2",
    text: "\"To'xtash taqiqlangan\" belgisi qaysi masofagacha amal qiladi?",
    hasImage: true,
    answers: [
      { id: "b1", shortcutLabel: "F1", text: "Keyingi chorrahagacha" },
      { id: "b2", shortcutLabel: "F2", text: "100 metrgacha" },
      { id: "b3", shortcutLabel: "F3", text: "Belgi o'rnatilgan hudud oxirigacha" },
    ],
    correctAnswerId: "b3",
  },
  {
    id: "q3",
    text: "Tormoz masofasi qanday omillarga bog'liq?",
    hasImage: false,
    answers: [
      { id: "c1", shortcutLabel: "F1", text: "Faqat tezlikka" },
      { id: "c2", shortcutLabel: "F2", text: "Tezlik, yo'l sirti va shina holatiga" },
      { id: "c3", shortcutLabel: "F3", text: "Faqat haydovchi tajribasiga" },
    ],
    correctAnswerId: "c2",
  },
  {
    id: "q4",
    text: "Imtihon rejimida noto'g'ri javob berilganda ekranda nima ko'rsatiladi?",
    hasImage: false,
    answers: [
      { id: "d1", shortcutLabel: "F1", text: "Darhol to'g'ri javob ko'rsatiladi" },
      { id: "d2", shortcutLabel: "F2", text: "Faqat \"javob qabul qilindi\" belgisi" },
      { id: "d3", shortcutLabel: "F3", text: "Ovozli ogohlantirish" },
    ],
    correctAnswerId: "d2",
  },
];

export const mockProfile = {
  name: "Aziz",
  isVip: false,
  streak: { current: 12, best: 21, todayDone: 6, dailyGoal: 10 },
  readinessPercent: 68,
};

// Category names match the 13 real, approved taxonomy codes
// (backend/cmd/convertavtoimtihon/categories.go) — not invented labels.
export const mockCategoryMastery = [
  { categoryName: "Yo'l belgilari va chizig'i", masteryPercent: 74 },
  { categoryName: "Chorrahalar va yo'l ustunligi", masteryPercent: 46 },
  { categoryName: "YHH, tez tibbiy yordam va tormozlash", masteryPercent: 28 },
  { categoryName: "To'xtash va to'xtab turish", masteryPercent: 83 },
];

export const proofStats = [
  { value: "1235", label: "savol" },
  { value: "61", label: "bilet" },
  { value: "13", label: "mavzu" },
  { value: "3", label: "til" },
];
```

- [ ] **Step 4: Run to verify it passes, then the full suite**

```bash
npm test -- mock-data && npm run typecheck && npm test
```

Expected: all clean.

- [ ] **Step 5: Commit**

```bash
cd "/home/sher/Рабочий стол/avtotest" && git add frontend/ && git commit -m "feat(frontend): static mock-data module for Phase A mockups"
```

---

### Task 7: Landing page mockup

**Files:**
- Modify: `frontend/src/app/(public)/page.tsx`
- Create: `frontend/src/app/(public)/demo-question-block.tsx`
- Create: `frontend/src/app/(public)/page.test.tsx`

**Interfaces:**
- Consumes: `Button` (Task 3), `QuestionCard`/`AnswerOption` (Task 4), `demoQuestion`/`proofStats` (Task 6).
- Produces: the real Landing page replacing Task 1's placeholder.

- [ ] **Step 1: Write the failing landing page test**

`frontend/src/app/(public)/page.test.tsx`:

```tsx
import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import LandingPage from "./page";

describe("LandingPage", () => {
  it("renders the hero CTA and all proof stats", () => {
    render(<LandingPage />);
    expect(screen.getByRole("button", { name: "Bepul boshlash" })).toBeInTheDocument();
    expect(screen.getByText("1235")).toBeInTheDocument();
    expect(screen.getByText("61")).toBeInTheDocument();
  });

  it("renders the interactive demo question", () => {
    render(<LandingPage />);
    expect(screen.getByText(/svetofor ishlamayapti/)).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd "/home/sher/Рабочий стол/avtotest/frontend" && npm test -- "app/(public)/page"`
Expected: FAIL — the placeholder page has neither the CTA text nor proof stats.

- [ ] **Step 3: Implement the interactive demo block**

`frontend/src/app/(public)/demo-question-block.tsx`:

```tsx
"use client";

import { useState } from "react";
import { QuestionCard } from "@/components/shared/question-card";
import { AnswerOption, type AnswerState } from "@/components/shared/answer-option";
import { demoQuestion } from "@/lib/mock-data";

export function DemoQuestionBlock() {
  const [selectedId, setSelectedId] = useState<string | null>(null);

  function stateFor(answerId: string): AnswerState {
    if (!selectedId) return "neutral";
    if (answerId === demoQuestion.correctAnswerId) return "correct";
    if (answerId === selectedId) return "incorrect";
    return "neutral";
  }

  return (
    <div className="mx-auto max-w-xl">
      <QuestionCard
        questionNumber={1}
        totalQuestions={1}
        text={demoQuestion.text}
        hasImage={demoQuestion.hasImage}
      />
      <div className="mt-4 flex flex-col gap-3">
        {demoQuestion.answers.map((answer) => (
          <AnswerOption
            key={answer.id}
            shortcutLabel={answer.shortcutLabel}
            text={answer.text}
            state={stateFor(answer.id)}
            onSelect={() => setSelectedId(answer.id)}
          />
        ))}
      </div>
    </div>
  );
}
```

- [ ] **Step 4: Implement the full landing page**

`frontend/src/app/(public)/page.tsx`:

```tsx
import { Button } from "@/components/ui/button";
import { proofStats } from "@/lib/mock-data";
import { DemoQuestionBlock } from "./demo-question-block";

const features = [
  { title: "FSRS o'quv dvigateli", text: "Har savol siz uchun aqlli takrorlash jadvali bilan due bo'ladi." },
  { title: "Tayyorlik %", text: "Imtihonga qanchalik tayyorligingizni real vaqtda ko'ring." },
  { title: "YHQ-havolali izohlar", text: "Har javobda huquqiy modda-havola bilan chuqur tushuntirish." },
  { title: "Real imtihon simulyatsiyasi", text: "20 savol, 25 daqiqa, 3-xatoda to'xtash — aynan rasmiy qoida." },
];

const steps = [
  { n: 1, title: "Ro'yxatdan o'ting", text: "Telefon raqamingiz bilan bir daqiqada." },
  { n: 2, title: "Bilet yeching", text: "Birinchi bilet har doim bepul." },
  { n: 3, title: "Tayyorlikni kuzating", text: "Statistikada zaif mavzularni ko'ring." },
];

const faqs = [
  { q: "Birinchi bilet chindan bepulmi?", a: "Ha, ro'yxatdan o'tgan har bir foydalanuvchi uchun 1-bilet doim bepul." },
  { q: "Savollar qayerdan olingan?", a: "Real imtihon savollari, litsenziyasi tasdiqlangan manbadan." },
  { q: "Necha tilda ishlaydi?", a: "uz-Latn, uz-Cyrl va rus tillarida." },
];

export default function LandingPage() {
  return (
    <main className="mx-auto max-w-5xl px-4 py-12">
      <section className="text-center">
        <h1 className="font-display text-4xl font-extrabold leading-tight md:text-6xl">
          Prava&apos;ni <span className="text-accent">oson</span> oling!
        </h1>
        <p className="mx-auto mt-4 max-w-xl text-lg text-muted-foreground">
          FSRS asosidagi aqlli o&apos;quv dvigateli bilan haydovchilik nazariy imtihoniga tayyorlaning.
        </p>
        <div className="mt-8 flex flex-col items-center gap-3 sm:flex-row sm:justify-center">
          <Button variant="game" size="lg">
            Bepul boshlash
          </Button>
          <span className="rounded-full border border-border px-4 py-1 text-sm text-muted-foreground">
            Ro&apos;yxatsiz sinab ko&apos;ring
          </span>
        </div>
      </section>

      <section className="mt-16">
        <h2 className="mb-6 text-center font-display text-2xl font-bold">Hozir sinab ko&apos;ring</h2>
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
        <h2 className="mb-6 text-center font-display text-2xl font-bold">Nega biz?</h2>
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
        <h2 className="mb-6 text-center font-display text-2xl font-bold">Qanday ishlaydi</h2>
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
        <h2 className="mb-6 text-center font-display text-2xl font-bold">Savol-javob</h2>
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
        AvtoTest — {new Date().getFullYear()}
      </footer>
    </main>
  );
}
```

- [ ] **Step 5: Run to verify it passes, then the full suite + build**

```bash
npm test -- "app/(public)/page" && npm run typecheck && npm test && npm run build
```

Expected: all clean.

- [ ] **Step 6: Commit**

```bash
cd "/home/sher/Рабочий стол/avtotest" && git add frontend/ && git commit -m "feat(frontend): Landing page mockup with interactive static demo question"
```

---

### Task 8: Dashboard page mockup

**Files:**
- Create: `frontend/src/app/(app)/dashboard/page.tsx`
- Create: `frontend/src/app/(app)/dashboard/page.test.tsx`

**Interfaces:**
- Consumes: `Button` (Task 3), `ResultRing` (Task 5), `mockProfile` (Task 6).
- Produces: the Dashboard mockup route at `/dashboard`.

- [ ] **Step 1: Write the failing dashboard test**

`frontend/src/app/(app)/dashboard/page.test.tsx`:

```tsx
import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import DashboardPage from "./page";

describe("DashboardPage", () => {
  it("renders the streak count and readiness ring", () => {
    render(<DashboardPage />);
    expect(screen.getByText("12")).toBeInTheDocument();
    expect(screen.getByText("68%")).toBeInTheDocument();
  });

  it("renders all four navigation cards", () => {
    render(<DashboardPage />);
    expect(screen.getByText("Biletlar")).toBeInTheDocument();
    expect(screen.getByText("Imtihon simulyatsiyasi")).toBeInTheDocument();
    expect(screen.getByText("Mashq")).toBeInTheDocument();
    expect(screen.getByText("Xatolar ustida ishlash")).toBeInTheDocument();
  });

  it("shows the free-plan badge when the user is not VIP", () => {
    render(<DashboardPage />);
    expect(screen.getByText("Bepul reja")).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd "/home/sher/Рабочий стол/avtotest/frontend" && npm test -- "app/(app)/dashboard"`
Expected: FAIL — module not found.

- [ ] **Step 3: Implement the dashboard page**

`frontend/src/app/(app)/dashboard/page.tsx`:

```tsx
import { Button } from "@/components/ui/button";
import { ResultRing } from "@/components/shared/result-ring";
import { mockProfile } from "@/lib/mock-data";
import { Flame } from "lucide-react";

const navCards = [
  { title: "Biletlar", desc: "61 ta bilet, ketma-ket ochiladi" },
  { title: "Imtihon simulyatsiyasi", desc: "20 savol, 25 daqiqa" },
  { title: "Mashq", desc: "Mavzu yoki belgi bo'yicha" },
  { title: "Xatolar ustida ishlash", desc: "FSRS asosida takrorlash" },
];

export default function DashboardPage() {
  const { name, isVip, streak, readinessPercent } = mockProfile;
  return (
    <main className="mx-auto max-w-4xl px-4 py-8">
      <header className="flex items-center justify-between">
        <div>
          <p className="text-muted-foreground">Xush kelibsiz,</p>
          <h1 className="font-display text-2xl font-bold">{name}</h1>
        </div>
        <span
          className={
            isVip
              ? "rounded-full bg-gold px-4 py-1 text-sm font-bold text-background"
              : "rounded-full border border-border px-4 py-1 text-sm text-muted-foreground"
          }
        >
          {isVip ? "Premium" : "Bepul reja"}
        </span>
      </header>

      <section className="mt-6 grid gap-4 sm:grid-cols-2">
        <div className="rounded-lg border border-border bg-card p-5">
          <div className="flex items-center gap-2">
            <Flame className="h-6 w-6 text-streak" />
            <span className="font-display text-2xl font-extrabold">{streak.current}</span>
            <span className="text-muted-foreground">kunlik streak</span>
          </div>
          <div className="mt-3 h-2 w-full overflow-hidden rounded-full bg-border">
            <div
              className="h-full rounded-full bg-streak"
              style={{ width: `${Math.min(100, (streak.todayDone / streak.dailyGoal) * 100)}%` }}
            />
          </div>
          <p className="mt-1 text-sm text-muted-foreground">
            Bugun: {streak.todayDone}/{streak.dailyGoal} savol
          </p>
        </div>

        <div className="flex items-center justify-center rounded-lg border border-border bg-card p-5">
          <ResultRing percent={readinessPercent} label="Tayyorlik" />
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

- [ ] **Step 4: Run to verify it passes, then the full suite**

```bash
npm test -- "app/(app)/dashboard" && npm run typecheck && npm test
```

Expected: all clean.

- [ ] **Step 5: Commit**

```bash
cd "/home/sher/Рабочий стол/avtotest" && git add frontend/ && git commit -m "feat(frontend): Dashboard page mockup"
```

---

### Task 9: Exam-screen mockup (with state switcher)

**Files:**
- Create: `frontend/src/app/(app)/exam-mockup/page.tsx`
- Create: `frontend/src/app/(app)/exam-mockup/page.test.tsx`

**Interfaces:**
- Consumes: `Button` (Task 3), `QuestionCard`/`AnswerOption` (Task 4), `CountdownTimer` (Task 5), `mockExamQuestions` (Task 6).
- Produces: the exam-screen mockup route at `/exam-mockup`, demonstrating all 4 required visual states in one page via a mockup-only state switcher (removed/replaced by real session-driven state in Phase B).

- [ ] **Step 1: Write the failing exam mockup test**

`frontend/src/app/(app)/exam-mockup/page.test.tsx`:

```tsx
import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import ExamMockupPage from "./page";

describe("ExamMockupPage", () => {
  it("shows no correct/incorrect feedback in the unanswered state", () => {
    render(<ExamMockupPage />);
    expect(screen.queryByTestId("answer-correct-icon")).not.toBeInTheDocument();
    expect(screen.queryByTestId("answer-incorrect-icon")).not.toBeInTheDocument();
  });

  it("reveals the correct answer and explanation when switched to the correct state", () => {
    render(<ExamMockupPage />);
    fireEvent.click(screen.getByRole("button", { name: "To'g'ri javob berilgan" }));
    expect(screen.getByTestId("answer-correct-icon")).toBeInTheDocument();
    expect(screen.getByText("MUHIM")).toBeInTheDocument();
  });

  it("never reveals correctness in the exam-hidden state, even after selecting an answer", () => {
    render(<ExamMockupPage />);
    fireEvent.click(screen.getByRole("button", { name: "Imtihon rejimi (feedback yashirin)" }));
    expect(screen.queryByTestId("answer-correct-icon")).not.toBeInTheDocument();
    expect(screen.queryByTestId("answer-incorrect-icon")).not.toBeInTheDocument();
  });

  it("shows a gold timer normally and a pulsating red timer in the exam-hidden (low-time) state", () => {
    render(<ExamMockupPage />);
    expect(screen.getByTestId("countdown-timer").className).toContain("text-gold");
    fireEvent.click(screen.getByRole("button", { name: "Imtihon rejimi (feedback yashirin)" }));
    expect(screen.getByTestId("countdown-timer").className).toContain("text-danger");
  });
});
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd "/home/sher/Рабочий стол/avtotest/frontend" && npm test -- exam-mockup`
Expected: FAIL — module not found.

- [ ] **Step 3: Implement the exam mockup page**

`frontend/src/app/(app)/exam-mockup/page.tsx`:

```tsx
"use client";

import { useState } from "react";
import { QuestionCard } from "@/components/shared/question-card";
import { AnswerOption, type AnswerState } from "@/components/shared/answer-option";
import { CountdownTimer } from "@/components/shared/countdown-timer";
import { Button } from "@/components/ui/button";
import { mockExamQuestions } from "@/lib/mock-data";

type Mode = "unanswered" | "correct" | "incorrect" | "exam-hidden";

const modeLabels: Record<Mode, string> = {
  unanswered: "Javobsiz",
  correct: "To'g'ri javob berilgan",
  incorrect: "Xato javob berilgan",
  "exam-hidden": "Imtihon rejimi (feedback yashirin)",
};

export default function ExamMockupPage() {
  const [mode, setMode] = useState<Mode>("unanswered");
  const question = mockExamQuestions[0];
  const wrongAnswerId = question.answers.find((a) => a.id !== question.correctAnswerId)!.id;
  const selectedAnswerId =
    mode === "correct" ? question.correctAnswerId : mode === "incorrect" ? wrongAnswerId : null;

  function stateFor(answerId: string): AnswerState {
    if (mode === "unanswered") return "neutral";
    if (mode === "exam-hidden") return answerId === selectedAnswerId ? "selected" : "neutral";
    if (answerId === question.correctAnswerId) return "correct";
    if (answerId === selectedAnswerId) return "incorrect";
    return "neutral";
  }

  return (
    <main className="mx-auto max-w-2xl px-4 py-8">
      {/* Mockup-only tooling: Phase B replaces this with real session state. */}
      <div className="mb-4 flex flex-wrap gap-2" role="group" aria-label="Mockup holatini tanlash">
        {(Object.keys(modeLabels) as Mode[]).map((m) => (
          <Button key={m} size="sm" variant={m === mode ? "default" : "outline"} onClick={() => setMode(m)}>
            {modeLabels[m]}
          </Button>
        ))}
      </div>

      <div className="mb-4 flex items-center justify-between">
        <span className="text-sm text-muted-foreground">1 / 20</span>
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

- [ ] **Step 4: Run to verify it passes, then the full suite + build**

```bash
npm test -- exam-mockup && npm run typecheck && npm test && npm run build
```

Expected: all clean.

- [ ] **Step 5: Commit**

```bash
cd "/home/sher/Рабочий стол/avtotest" && git add frontend/ && git commit -m "feat(frontend): exam-screen mockup demonstrating all 4 visual states"
```

---

### Task 10: Final verification + docs

**Files:**
- Create: `frontend/README.md`
- Modify: `/home/sher/Рабочий стол/avtotest/README.md` (Frontend section)

**Interfaces:** none — this task only verifies and documents what Tasks 1-9 built.

- [ ] **Step 1: Write `frontend/README.md`**

```markdown
# AvtoTest Frontend

Next.js 14 (App Router) + TypeScript + Tailwind CSS. Phase A status: design
system + 3 static mockup pages (no real API calls yet — see
`docs/superpowers/specs/2026-07-22-nextjs-frontend-foundation-design.md`).

## Dev

\`\`\`bash
npm install
npm run dev        # http://localhost:3000
npm run lint
npm run typecheck
npm test
npm run build
\`\`\`

## Phase A mockup routes

- `/` — Landing (hero, interactive static demo question, proof stats, features, FAQ)
- `/dashboard` — Dashboard (streak, readiness ring, 4 nav cards)
- `/exam-mockup` — Exam screen, with a state switcher demonstrating unanswered /
  correct / incorrect / exam-hidden (anti-cheat, no feedback) visual states

None of these call the real Go backend yet — all data comes from
`src/lib/mock-data.ts`. Phase B wires real auth, TanStack Query/Zustand, and
the remaining pages.
```

- [ ] **Step 2: Update the root README's Frontend section**

Modify `/home/sher/Рабочий стол/avtotest/README.md` — append to the existing "## Frontend" section (after the paragraph ending in `AVTOTEST-MASTER-PROMPT.txt`.):

```markdown

**Phase A (2026-07-22):** skelet + dizayn-tizim + 3 statik mockup sahifa
tayyor (`frontend/`, real API'ga ulanmagan). Ishga tushirish:
`cd frontend && npm install && npm run dev`. Tafsilot:
`docs/superpowers/specs/2026-07-22-nextjs-frontend-foundation-design.md`
va `frontend/README.md`.
```

- [ ] **Step 3: Run the complete verification suite**

```bash
cd "/home/sher/Рабочий стол/avtotest/frontend" && npm run lint && npm run typecheck && npm test && npm run build
```

Expected: every command exits 0, no warnings in test output.

- [ ] **Step 4: Manual visual check (do this yourself, report the outcome — do not skip)**

```bash
npm run dev
```

Open `http://localhost:3000/`, `http://localhost:3000/dashboard`, `http://localhost:3000/exam-mockup` in a browser. For each: toggle dark/light (there is no header yet wiring `<ThemeToggle />` into these pages — temporarily render `<ThemeToggle />` at the top of one page to flip themes while checking, then remove that temporary line before committing), and resize the window to a narrow (~375px) mobile width. Confirm: no horizontal scrollbar, no illegible/low-contrast text in either theme, the pill-button 3D press effect is visible on click, the exam-mockup's 4 states all render as designed. Stop the dev server (Ctrl-C) when done.

- [ ] **Step 5: Commit**

```bash
cd "/home/sher/Рабочий стол/avtotest" && git add frontend/README.md README.md && git commit -m "docs(frontend): Phase A README + verification"
```

---

## Self-Review Notes (for whoever executes this plan)

- Every task's deliverable is independently testable and buildable; no task leaves `npm run build` broken for a later task to fix.
- No route stub exists for anything this plan doesn't implement (`login`, `variants`, `practice`, etc.) — matches the "halol placeholder" rule.
- `AnswerOption` has no `isCorrect` prop anywhere in this plan — the anti-cheat property is structural, not just convention, mirrored from the real backend's session package.
- `@tanstack/react-query` and `zustand` are installed (Task 1) and never imported by name in any of Tasks 2-10 — intentional, per the approved spec.
