"use client";

import { useEffect, useState } from "react";
import { useTheme } from "next-themes";
import { useTranslations } from "next-intl";
import { Moon, Sun } from "lucide-react";

type Props = {
  /** Smaller control for cramped mobile drawers. */
  size?: "sm" | "md";
};

export function ThemeToggle({ size = "md" }: Props) {
  const { theme, setTheme } = useTheme();
  const t = useTranslations("ThemeToggle");
  const [mounted, setMounted] = useState(false);
  const box = size === "sm" ? "h-9 w-9 rounded-xl" : "h-11 w-11 rounded-full";

  useEffect(() => {
    setMounted(true);
  }, []);

  if (!mounted) {
    return <span className={`inline-block ${box}`} aria-hidden />;
  }

  const isDark = theme === "dark";

  return (
    <button
      type="button"
      aria-label={isDark ? t("toLight") : t("toDark")}
      onClick={() => setTheme(isDark ? "light" : "dark")}
      className={`flex items-center justify-center border border-border bg-card shadow-raised-sm transition-[transform,box-shadow] duration-150 hover:-translate-y-0.5 active:translate-y-0.5 active:shadow-none ${box}`}
    >
      {isDark ? (
        <Sun data-testid="theme-toggle-sun" className="h-4 w-4" />
      ) : (
        <Moon data-testid="theme-toggle-moon" className="h-4 w-4" />
      )}
    </button>
  );
}
