import { locales, type Locale } from "@/i18n/config";
import { isTelegramMiniApp } from "./web-app";

/**
 * The Mini App's language, kept apart from NEXT_LOCALE on purpose. The bot
 * always opens /uz-Latn/tg, and next-intl's middleware rewrites NEXT_LOCALE
 * to the locale of every document it serves — so by the time /tg runs the
 * cookie says "uz-Latn" whatever the learner chose, and cannot tell a first
 * open from a deliberate choice.
 */
export const TG_LOCALE_KEY = "tg-locale";

function isLocale(value: string | null): value is Locale {
  return value !== null && (locales as readonly string[]).includes(value);
}

function store(locale: Locale): void {
  try {
    localStorage.setItem(TG_LOCALE_KEY, locale);
  } catch {
    /* storage blocked: every open falls back to language_code */
  }
}

/**
 * The locale /tg should render: the last language picked inside the Mini
 * App, else (first open) Russian for a Russian Telegram client and the URL's
 * locale for everyone else. The first-open answer is remembered.
 */
export function resolveTelegramLocale(current: Locale, languageCode: string | undefined): Locale {
  let stored: string | null = null;
  try {
    stored = localStorage.getItem(TG_LOCALE_KEY);
  } catch {
    /* treated as a first open */
  }
  if (isLocale(stored)) return stored;
  const chosen: Locale = languageCode === "ru" ? "ru" : current;
  store(chosen);
  return chosen;
}

/** Records an in-app language switch so the next bot open honours it. No-op on the website. */
export function rememberTelegramLocale(locale: Locale): void {
  if (isTelegramMiniApp()) store(locale);
}
