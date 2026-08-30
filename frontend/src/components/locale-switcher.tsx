"use client";

import { startTransition, useEffect, useId, useRef, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import { usePathname, useRouter } from "@/i18n/navigation";
import type { Locale } from "@/i18n/config";
import { ChevronDown } from "lucide-react";
import { useTransitionNavigate } from "@/components/layout/view-transitions";

const LOCALES = [
  { code: "uz-Latn" as const, labelKey: "languageUzLatn" as const },
  { code: "uz-Cyrl" as const, labelKey: "languageUzCyrl" as const },
  { code: "ru" as const, labelKey: "languageRu" as const },
];

type Props = {
  /** Compact chips for sticky headers (default). */
  size?: "sm" | "md";
  /**
   * Single control + dropdown — use in cramped mobile headers
   * (landing) where three chips steal the brand row.
   */
  compact?: boolean;
  /** Stretch chips across the available width (sidebar footer). */
  fill?: boolean;
  /** Soften chrome when nested inside a shared toolbar. */
  embedded?: boolean;
  /** Where the compact menu opens relative to the trigger. */
  menuPlacement?: "top" | "bottom";
  /** Horizontal anchoring for the compact menu. */
  menuAlign?: "start" | "end";
  className?: string;
};

/**
 * Soft-switches locale via next-intl navigation (cookie + path prefix)
 * without scrolling to top. Shared by landing, header, and sidebar.
 */
export function LocaleSwitcher({
  size = "sm",
  compact = false,
  fill = false,
  embedded = false,
  menuPlacement = "bottom",
  menuAlign = "end",
  className = "",
}: Props) {
  const currentLocale = useLocale();
  const t = useTranslations("LocaleSwitcher");
  const pathname = usePathname();
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);
  const listId = useId();
  const withTransition = useTransitionNavigate();
  const warmedRef = useRef(new Set<Locale>());

  /**
   * A language switch is a route change like any other, so it earns the same
   * cross-fade — but only if the other locale is already loaded. Reaching for
   * the switcher warms them, which is early enough: the pointer arrives well
   * before the click does.
   */
  const warmLocales = () => {
    for (const { code } of LOCALES) {
      if (code === currentLocale || warmedRef.current.has(code)) continue;
      warmedRef.current.add(code);
      router.prefetch(pathname, { locale: code });
    }
  };

  const handleLanguageChange = (newLocale: Locale) => {
    if (newLocale === currentLocale) return;
    const query =
      typeof window !== "undefined"
        ? Object.fromEntries(new URLSearchParams(window.location.search))
        : {};
    const navigate = () =>
      router.replace(
        Object.keys(query).length > 0 ? { pathname, query } : pathname,
        { locale: newLocale, scroll: false }
      );

    withTransition(
      // No React startTransition inside a view transition: both exist to keep
      // the old UI on screen while the new one prepares, and nesting them
      // pushes the commit to low priority. Measured on production, that alone
      // took the switch past the freeze limit every time, so the cross-fade was
      // always dropped. Outside one it still defers, as before.
      (driven) => (driven ? navigate() : startTransition(navigate)),
      { warm: warmedRef.current.has(newLocale) },
    );
    setOpen(false);
  };

  useEffect(() => {
    if (!open) return;
    const onPointer = (event: MouseEvent | TouchEvent) => {
      const el = rootRef.current;
      if (el && !el.contains(event.target as Node)) setOpen(false);
    };
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") setOpen(false);
    };
    document.addEventListener("mousedown", onPointer);
    document.addEventListener("touchstart", onPointer);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", onPointer);
      document.removeEventListener("touchstart", onPointer);
      document.removeEventListener("keydown", onKey);
    };
  }, [open]);

  const current = LOCALES.find((l) => l.code === currentLocale) ?? LOCALES[0];

  if (compact) {
    const menuPos =
      menuPlacement === "top"
        ? "bottom-[calc(100%+0.35rem)]"
        : "top-[calc(100%+0.35rem)]";
    const menuSide = menuAlign === "start" ? "left-0" : "right-0";

    return (
      <div
        ref={rootRef}
        className={`relative z-20 ${className}`}
        onPointerOver={warmLocales}
        onFocus={warmLocales}
      >
        <button
          type="button"
          aria-label={t("languageSwitcher")}
          aria-haspopup="listbox"
          aria-expanded={open}
          aria-controls={listId}
          onClick={() => setOpen((v) => !v)}
          className="inline-flex h-9 min-w-10 items-center gap-1 rounded-xl border border-border/80 bg-card px-2.5 text-xs font-bold text-foreground shadow-raised-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        >
          {t(current.labelKey)}
          <ChevronDown
            aria-hidden="true"
            className={`h-3.5 w-3.5 text-muted-foreground transition-transform ${
              open ? (menuPlacement === "top" ? "rotate-0" : "rotate-180") : menuPlacement === "top" ? "rotate-180" : ""
            }`}
          />
        </button>
        {open ? (
          <ul
            id={listId}
            role="listbox"
            aria-label={t("languageSwitcher")}
            className={`absolute ${menuPos} ${menuSide} z-50 min-w-[7.5rem] max-w-[calc(100vw-2rem)] overflow-hidden rounded-xl border border-border bg-card p-1 shadow-raised-sm`}
          >
            {LOCALES.map((lang) => (
              <li key={lang.code} role="option" aria-selected={currentLocale === lang.code}>
                <button
                  type="button"
                  onClick={() => handleLanguageChange(lang.code)}
                  className={`flex w-full items-center rounded-lg px-3 py-2 text-left text-sm font-bold transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring ${
                    currentLocale === lang.code
                      ? "bg-accent text-accent-foreground"
                      : "text-muted-foreground hover:bg-background hover:text-foreground"
                  }`}
                >
                  {t(lang.labelKey)}
                </button>
              </li>
            ))}
          </ul>
        ) : null}
      </div>
    );
  }

  const chip =
    size === "md"
      ? "min-h-11 rounded-md px-2.5 text-sm font-bold md:px-3"
      : "min-h-10 min-w-10 rounded px-2 py-1 text-xs font-bold";
  const shell = embedded
    ? "border-0 bg-transparent p-0 shadow-none"
    : "rounded-lg border border-border/80 bg-card p-0.5 shadow-raised-sm";

  return (
    <div
      role="group"
      aria-label={t("languageSwitcher")}
      className={`flex gap-0.5 ${fill ? "w-full" : ""} ${shell} ${className}`}
      onPointerOver={warmLocales}
      onFocus={warmLocales}
    >
      {LOCALES.map((lang) => (
        <button
          type="button"
          key={lang.code}
          onClick={() => handleLanguageChange(lang.code)}
          aria-pressed={currentLocale === lang.code}
          className={`${chip} ${fill ? "min-w-0 flex-1" : ""} transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring ${
            currentLocale === lang.code
              ? "bg-accent text-accent-foreground"
              : "text-muted-foreground hover:bg-background/80 hover:text-foreground"
          }`}
        >
          {t(lang.labelKey)}
        </button>
      ))}
    </div>
  );
}
