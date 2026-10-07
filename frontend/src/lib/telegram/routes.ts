import { defaultLocale, locales, type Locale } from "@/i18n/config";

/**
 * Screens where Telegram's BackButton stays hidden: the phone's bottom tabs
 * (sidebar.tsx `bottomTabs`: dashboard, tickets, practice, exam), the profile
 * — its phone layout runs its own sub-screens with their own back arrows, and
 * a Telegram Back there would leave the whole profile instead of closing the
 * open panel — and /tg, the Mini App's entry. Everything else is a drill-down.
 * Arena is not a tab (it lives in the menu), so it shows Back.
 */
const TAB_ROOTS = new Set(["dashboard", "tickets", "practice", "exam", "profile", "tg"]);

function segments(pathname: string): string[] {
  return pathname.split("/").filter(Boolean); // [locale, ...rest]
}

export function localeOf(pathname: string): Locale {
  const first = segments(pathname)[0];
  return (locales as readonly string[]).includes(first ?? "") ? (first as Locale) : defaultLocale;
}

/** Telegram's BackButton is hidden on the tab roots, shown elsewhere. */
export function isTabRoot(pathname: string): boolean {
  const [, first, second] = segments(pathname);
  if (!first) return true;
  return TAB_ROOTS.has(first) && second === undefined;
}

/**
 * A swipe-down or tap on X mid-test would lose the attempt; ask first. The
 * runners are (session)/session/[id] (every mode, exams included) and
 * (session)/practice/memorize/[code]; /session/start only creates a session
 * and redirects, so there is nothing to lose there yet.
 */
export function needsClosingGuard(pathname: string): boolean {
  const [, first, second] = segments(pathname);
  if (first === "session") return second !== undefined && second !== "start";
  return first === "practice" && second === "memorize" && segments(pathname)[3] !== undefined;
}
