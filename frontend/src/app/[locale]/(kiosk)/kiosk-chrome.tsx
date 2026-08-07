"use client";

import { usePathname } from "next/navigation";
import { LocaleSwitcher } from "@/components/locale-switcher";
import { ThemeToggle } from "@/components/theme-toggle";

/**
 * Language and light/dark controls for the classroom kiosk.
 *
 * These belong to whoever is sitting at the PC, not to the school's admin.
 * The installer used to bake a locale into each school's .exe, which meant a
 * Russian-speaking student on a uz-Latn machine had no way out — the kiosk has
 * no profile page and no settings screen to fall back on. The agent's embedded
 * locale is now only the URL it opens first; everything after that is decided
 * here, in front of the student.
 *
 * LocaleSwitcher swaps the locale prefix and keeps the rest of the path, so
 * /uz-Latn/station/practice becomes /ru/station/practice and the student stays
 * inside the kiosk. ThemeToggle writes next-themes' localStorage key, which
 * the pre-paint script in [locale]/layout.tsx reads on every later boot, so a
 * classroom PC keeps the theme it was set to.
 */
export function KioskChrome() {
  const pathname = usePathname();

  // A *running* exam is full-screen and carries its own exit, bookmark,
  // language and fullscreen controls in a single row; a second floating bar
  // over it would cover the question and hand a student a way to change locale
  // mid-question through a different code path than the one the session page
  // already owns. /station/session/start is the ordinary pre-exam page and
  // keeps the chrome — that is the last screen where picking a language is
  // still free.
  if (/\/station\/session\/(?!start(?:\/|$))/.test(pathname)) return null;

  return (
    <div
      data-testid="kiosk-chrome"
      className="pointer-events-none fixed right-3 top-3 z-40 flex items-center gap-2 sm:right-4 sm:top-4"
    >
      <div className="pointer-events-auto">
        <LocaleSwitcher size="sm" />
      </div>
      <div className="pointer-events-auto">
        <ThemeToggle size="sm" />
      </div>
    </div>
  );
}
