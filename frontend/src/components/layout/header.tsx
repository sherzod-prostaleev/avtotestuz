"use client";

import { useLocale, useTranslations } from "next-intl";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { BrandLogo } from "@/components/brand/brand-logo";
import { ThemeToggle } from "@/components/theme-toggle";
import { useUserStats } from "@/hooks/use-user-stats";
import {
  AlertTriangle,
  BookOpen,
  Crown,
  Flame,
  LayoutDashboard,
  Signpost,
  Target,
  User,
} from "lucide-react";

export function Header() {
  const currentLocale = useLocale();
  const t = useTranslations("Header");
  const pathname = usePathname();
  const router = useRouter();
  const { streak, entitlement, loading } = useUserStats();

  const isVip = entitlement?.is_vip ?? false;
  const currentStreak = streak?.current_streak ?? 0;

  const handleLanguageChange = (newLocale: string) => {
    if (newLocale === currentLocale) return;
    const newPath = pathname.replace(`/${currentLocale}`, `/${newLocale}`);
    router.push(newPath);
  };

  const navLinks = [
    { href: `/${currentLocale}/dashboard`, label: t("navDashboard"), icon: LayoutDashboard },
    { href: `/${currentLocale}/tickets`, label: t("navTickets"), icon: BookOpen },
    { href: `/${currentLocale}/signs`, label: t("navSigns"), icon: Signpost },
    { href: `/${currentLocale}/practice`, label: t("navPractice"), icon: Target },
    { href: `/${currentLocale}/mistakes`, label: t("navMistakes"), icon: AlertTriangle },
  ];

  return (
    <header className="sticky top-0 z-40 w-full border-b border-border/80 bg-background/80 backdrop-blur-md">
      <div className="mx-auto flex h-16 max-w-6xl items-center justify-between px-4">
        {/* Brand Logo */}
        <Link
          href={`/${currentLocale}/dashboard`}
          aria-label={t("brandDashboardLabel")}
          className="flex items-center gap-3 font-display text-2xl font-black tracking-tight text-foreground"
        >
          <BrandLogo size={40} className="h-10 w-10 rounded-2xl object-cover" />
          <span>{t("brandName")}</span>
        </Link>

        {/* Navigation Tabs (Desktop) */}
        <nav className="hidden md:flex items-center gap-1">
          {navLinks.map((link) => {
            const Icon = link.icon;
            const isActive = pathname === link.href;
            return (
              <Link
                key={link.href}
                href={link.href}
                aria-current={isActive ? "page" : undefined}
                className={`flex items-center gap-1.5 rounded-lg px-3 py-2 text-xs font-bold transition-all ${
                  isActive
                    ? "bg-accent/15 text-accent"
                    : "text-muted-foreground hover:bg-card hover:text-foreground"
                }`}
              >
                <Icon aria-hidden="true" className="h-4 w-4" />
                {link.label}
              </Link>
            );
          })}
        </nav>

        {/* Right Action Icons & Profile */}
        <div className="flex items-center gap-3">
          {/* Streak Flame Pill */}
          <div
            className="flex items-center gap-1.5 rounded-full border border-streak/30 bg-streak/10 px-3 py-1 text-xs font-extrabold text-streak"
            aria-label={t("streakLabel", { count: currentStreak })}
          >
            <Flame aria-hidden="true" className="h-4 w-4 animate-flame text-streak" />
            <span>{currentStreak}</span>
          </div>

          {/* VIP Badge — stays in the neutral/free style until the
              entitlement fetch resolves, so a VIP user never sees the "Free"
              claim flash before it flips to "VIP". */}
          <Link
            href={`/${currentLocale}/premium`}
            aria-label={!loading && isVip ? t("vipActive") : t("openPremium")}
          >
            <div
              className={`flex items-center gap-1 rounded-full px-2.5 py-1 text-xs font-bold transition-transform hover:scale-105 ${
                !loading && isVip
                  ? "bg-gold/20 text-gold border border-gold/40"
                  : "bg-card border border-border text-muted-foreground hover:border-gold/50"
              }`}
            >
              <Crown aria-hidden="true" className={`h-3.5 w-3.5 ${!loading && isVip ? "text-gold" : ""}`} />
              {loading ? (
                <span
                  aria-hidden="true"
                  className="hidden h-2.5 w-8 animate-pulse rounded-full bg-border/70 sm:inline-block"
                />
              ) : (
                <span className="hidden sm:inline">{isVip ? t("vip") : t("free")}</span>
              )}
            </div>
          </Link>

          {/* Language Switcher */}
          <div
            role="group"
            aria-label={t("languageSwitcher")}
            className="flex gap-0.5 rounded-lg border border-border/80 bg-card p-0.5"
          >
            {[
              { code: "uz-Latn", label: t("languageUzLatn") },
              { code: "uz-Cyrl", label: t("languageUzCyrl") },
              { code: "ru", label: t("languageRu") },
            ].map((lang) => (
              <button
                type="button"
                key={lang.code}
                onClick={() => handleLanguageChange(lang.code)}
                aria-pressed={currentLocale === lang.code}
                className={`rounded px-2 py-0.5 text-[11px] font-bold transition-all min-h-9 min-w-9 ${
                  currentLocale === lang.code
                    ? "bg-accent text-accent-foreground"
                    : "text-muted-foreground hover:text-foreground"
                }`}
              >
                {lang.label}
              </button>
            ))}
          </div>

          {/* Theme Toggle */}
          <ThemeToggle />

          {/* Profile Pill */}
          <Link href={`/${currentLocale}/profile`} aria-label={t("openProfile")}>
            <div className="flex h-9 w-9 items-center justify-center rounded-full border border-border bg-card text-muted-foreground transition-all hover:border-accent hover:text-accent">
              <User aria-hidden="true" className="h-4 w-4" />
            </div>
          </Link>
        </div>
      </div>
    </header>
  );
}
