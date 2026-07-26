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
    return <span className="inline-block h-11 w-11" aria-hidden />;
  }

  const isDark = theme === "dark";

  return (
    <button
      type="button"
      aria-label={isDark ? t("toLight") : t("toDark")}
      onClick={() => setTheme(isDark ? "light" : "dark")}
      className="flex h-11 w-11 items-center justify-center rounded-full border border-border bg-card shadow-raised-sm transition-[transform,box-shadow] duration-150 hover:-translate-y-0.5 active:translate-y-0.5 active:shadow-none"
    >
      {isDark ? (
        <Sun data-testid="theme-toggle-sun" className="h-4 w-4" />
      ) : (
        <Moon data-testid="theme-toggle-moon" className="h-4 w-4" />
      )}
    </button>
  );
}
