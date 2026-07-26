"use client";

import { useEffect, useState } from "react";
import { useTheme } from "next-themes";
import { useTranslations } from "next-intl";
import { Moon, Sun } from "lucide-react";

type Props = {
  /** Smaller control for cramped mobile drawers. */
  size?: "sm" | "md";
  /** Soften chrome when nested inside a shared toolbar. */
  embedded?: boolean;
  className?: string;
};

export function ThemeToggle({ size = "md", embedded = false, className = "" }: Props) {
  const { theme, setTheme } = useTheme();
  const t = useTranslations("ThemeToggle");
  const [mounted, setMounted] = useState(false);
  const box = embedded
    ? size === "sm"
      ? "h-10 w-10 min-h-10 min-w-10 rounded-lg"
      : "h-11 w-11 min-h-11 min-w-11 rounded-lg"
    : size === "sm"
      ? "h-9 w-9 rounded-xl"
      : "h-11 w-11 rounded-full";
  const chrome = embedded
    ? "border border-transparent bg-transparent shadow-none hover:bg-background hover:text-foreground"
    : "border border-border bg-card shadow-raised-sm hover:-translate-y-0.5 active:translate-y-0.5 active:shadow-none";

  useEffect(() => {
    setMounted(true);
  }, []);

  if (!mounted) {
    return <span className={`inline-block ${box} ${className}`} aria-hidden />;
  }

  const isDark = theme === "dark";

  return (
    <button
      type="button"
      aria-label={isDark ? t("toLight") : t("toDark")}
      onClick={() => setTheme(isDark ? "light" : "dark")}
      className={`flex shrink-0 items-center justify-center text-foreground transition-[transform,box-shadow,background-color,color] duration-150 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring ${chrome} ${box} ${className}`}
    >
      {isDark ? (
        <Sun data-testid="theme-toggle-sun" className="h-4 w-4" />
      ) : (
        <Moon data-testid="theme-toggle-moon" className="h-4 w-4" />
      )}
    </button>
  );
}
