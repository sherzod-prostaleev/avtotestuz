"use client";

import { startTransition } from "react";
import { useLocale, useTranslations } from "next-intl";
import { usePathname, useRouter } from "@/i18n/navigation";
import type { Locale } from "@/i18n/config";

const LOCALES = [
  { code: "uz-Latn" as const, labelKey: "languageUzLatn" as const },
  { code: "uz-Cyrl" as const, labelKey: "languageUzCyrl" as const },
  { code: "ru" as const, labelKey: "languageRu" as const },
];

type Props = {
  /** Compact chips for sticky headers (default). */
  size?: "sm" | "md";
  className?: string;
};

/**
 * Soft-switches locale via next-intl navigation (cookie + path prefix)
 * without scrolling to top. Shared by landing, header, and sidebar.
 */
export function LocaleSwitcher({ size = "sm", className = "" }: Props) {
  const currentLocale = useLocale();
  const t = useTranslations("LocaleSwitcher");
  const pathname = usePathname();
  const router = useRouter();

  const handleLanguageChange = (newLocale: Locale) => {
    if (newLocale === currentLocale) return;
    const query =
      typeof window !== "undefined"
        ? Object.fromEntries(new URLSearchParams(window.location.search))
        : {};
    startTransition(() => {
      router.replace(
        Object.keys(query).length > 0 ? { pathname, query } : pathname,
        { locale: newLocale, scroll: false }
      );
    });
  };

  const chip =
    size === "md"
      ? "min-h-10 rounded-md px-3 text-[11px] font-bold"
      : "min-h-9 min-w-9 rounded px-2 py-0.5 text-[11px] font-bold";

  return (
    <div
      role="group"
      aria-label={t("languageSwitcher")}
      className={`flex gap-0.5 rounded-lg border border-border/80 bg-card p-0.5 ${className}`}
    >
      {LOCALES.map((lang) => (
        <button
          type="button"
          key={lang.code}
          onClick={() => handleLanguageChange(lang.code)}
          aria-pressed={currentLocale === lang.code}
          className={`${chip} transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring ${
            currentLocale === lang.code
              ? "bg-accent text-accent-foreground"
              : "text-muted-foreground hover:text-foreground"
          }`}
        >
          {t(lang.labelKey)}
        </button>
      ))}
    </div>
  );
}
