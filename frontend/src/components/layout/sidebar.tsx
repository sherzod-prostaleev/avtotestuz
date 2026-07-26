"use client";

import { useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { BrandLogo } from "@/components/brand/brand-logo";
import { LocaleSwitcher } from "@/components/locale-switcher";
import { ThemeToggle } from "@/components/theme-toggle";
import { TrialCountdown } from "@/components/shared/trial-countdown";
import { useUserStats } from "@/hooks/use-user-stats";
import { OFFICIAL_TICKET_COUNT } from "@/lib/content-counts";
import {
  LayoutDashboard,
  BookOpen,
  Award,
  Target,
  Signpost,
  AlertTriangle,
  BarChart3,
  User,
  Crown,
  Flame,
  Menu,
  X,
  Bookmark,
  Trophy,
  ChevronDown,
  Swords,
} from "lucide-react";

type NavLink = {
  href: string;
  label: string;
  icon: typeof LayoutDashboard;
  isGold?: boolean;
};

export function Sidebar() {
  const currentLocale = useLocale();
  const t = useTranslations("Sidebar");
  const pathname = usePathname();
  const [mobileOpen, setMobileOpen] = useState(false);

  const { streak, entitlement, user, loading } = useUserStats();

  const isVip = entitlement?.is_vip ?? false;
  const currentStreak = streak?.current_streak ?? 0;
  const userName = user?.name || t("userFallback");

  const primaryLinks: NavLink[] = [
    { href: `/${currentLocale}/dashboard`, label: t("navDashboard"), icon: LayoutDashboard },
    { href: `/${currentLocale}/tickets`, label: t("navTickets", { count: OFFICIAL_TICKET_COUNT }), icon: BookOpen },
    { href: `/${currentLocale}/practice`, label: t("navPractice"), icon: Target },
    { href: `/${currentLocale}/arena`, label: t("navArena"), icon: Swords },
    { href: `/${currentLocale}/session/start?mode=exam`, label: t("navExam"), icon: Award },
    { href: `/${currentLocale}/signs`, label: t("navSigns"), icon: Signpost },
    { href: `/${currentLocale}/premium`, label: t("navPremium"), icon: Crown, isGold: true },
    // Profile/settings was buried under "Yana" — users reported they could not find/open it.
    { href: `/${currentLocale}/profile`, label: t("navProfile"), icon: User },
  ];

  const moreLinks: NavLink[] = [
    { href: `/${currentLocale}/mistakes`, label: t("navMistakes"), icon: AlertTriangle },
    { href: `/${currentLocale}/saved`, label: t("navSaved"), icon: Bookmark },
    { href: `/${currentLocale}/leaderboard`, label: t("navLeaderboard"), icon: Trophy },
    { href: `/${currentLocale}/stats`, label: t("navStats"), icon: BarChart3 },
  ];

  const moreActive = moreLinks.some((link) => {
    const pathOnly = link.href.split("?")[0];
    return pathname === pathOnly || pathname.startsWith(`${pathOnly}/`);
  });
  const [moreOpen, setMoreOpen] = useState(false);
  const showMore = moreOpen || moreActive;

  const isLinkActive = (href: string) => {
    const pathOnly = href.split("?")[0];
    return pathname === pathOnly || (href.includes("session") && pathname.includes("session"));
  };

  const renderLink = (link: NavLink) => {
    const Icon = link.icon;
    const isActive = isLinkActive(link.href);
    return (
      <Link
        key={link.href}
        href={link.href}
        onClick={() => setMobileOpen(false)}
        className={`sidebar-link focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring ${
          isActive
            ? "sidebar-link-active"
            : link.isGold
              ? "border border-gold/25 bg-gold/10 text-gold shadow-raised-sm hover:bg-gold/15"
              : "sidebar-link-inactive"
        }`}
      >
        <Icon aria-hidden="true" className="h-4 w-4 shrink-0" />
        <span className="leading-tight">{link.label}</span>
      </Link>
    );
  };

  return (
    <>
      <div
        className="sticky top-0 z-40 flex h-14 w-full items-center justify-between border-b border-border bg-background/95 px-4 backdrop-blur-md md:hidden"
        style={{ paddingTop: "env(safe-area-inset-top)" }}
      >
        <Link
          href={`/${currentLocale}/dashboard`}
          className="flex min-h-11 items-center gap-2.5 font-display text-xl font-bold text-foreground"
        >
          <BrandLogo size={36} className="h-9 w-9 rounded-2xl object-cover" />
          <span className="font-black">{t("brandName")}</span>
        </Link>

        <div className="flex items-center gap-2">
          <div className="flex min-h-11 items-center gap-1.5 rounded-xl border border-streak/30 bg-streak/10 px-3 text-sm font-bold text-streak">
            <Flame aria-hidden="true" className="h-4 w-4 animate-flame" />
            <span className="tabular-nums">{currentStreak}</span>
          </div>

          <button
            type="button"
            aria-label={mobileOpen ? t("closeMenu") : t("openMenu")}
            onClick={() => setMobileOpen(!mobileOpen)}
            className="flex h-11 w-11 items-center justify-center rounded-xl border border-border bg-card text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          >
            {mobileOpen ? (
              <X aria-hidden="true" className="h-5 w-5" />
            ) : (
              <Menu aria-hidden="true" className="h-5 w-5" />
            )}
          </button>
        </div>
      </div>

      {mobileOpen && (
        <button
          type="button"
          aria-label={t("closeMenu")}
          onClick={() => setMobileOpen(false)}
          className="fixed inset-0 z-40 bg-black/60 backdrop-blur-sm md:hidden"
        />
      )}

      <aside
        className={`fixed bottom-0 left-0 top-0 z-50 flex w-[min(18.5rem,90vw)] flex-col overflow-hidden border-r border-border bg-card p-3 shadow-[6px_0_28px_-18px_hsl(var(--elev-ambient)/0.65)] transition-transform duration-300 md:w-64 md:translate-x-0 ${
          mobileOpen ? "translate-x-0" : "-translate-x-full md:translate-x-0"
        }`}
        style={{
          paddingTop: "max(0.75rem, env(safe-area-inset-top))",
          paddingBottom: "max(0.75rem, env(safe-area-inset-bottom))",
        }}
      >
        {/* Header — fixed */}
        <div className="shrink-0 space-y-2.5">
          <div className="flex items-center justify-between px-1">
            <Link
              href={`/${currentLocale}/dashboard`}
              className="flex items-center gap-2 font-display text-lg font-black text-foreground"
            >
              <BrandLogo size={36} className="h-9 w-9 rounded-2xl object-cover" />
              <span>{t("brandName")}</span>
            </Link>

            <button
              type="button"
              aria-label={t("closeMenu")}
              onClick={() => setMobileOpen(false)}
              className="flex h-10 w-10 items-center justify-center rounded-xl text-muted-foreground hover:bg-background hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring md:hidden"
            >
              <X aria-hidden="true" className="h-5 w-5" />
            </button>
          </div>

          <div className="sidebar-panel px-3 py-2">
            <div className="flex items-center justify-between gap-2 text-sm font-extrabold">
              <span className="flex items-center gap-1.5 text-streak">
                <Flame aria-hidden="true" className="h-4 w-4 animate-flame" />
                {t("streakCount", { count: currentStreak })}
              </span>
              {loading ? (
                <span aria-hidden="true" className="h-5 w-14 animate-pulse rounded-full bg-border/60" />
              ) : isVip ? (
                <span className="rounded-md border border-gold/30 bg-gold/15 px-2 py-0.5 text-xs font-extrabold text-gold">
                  {t("vipBadge")}
                </span>
              ) : (
                <Link href={`/${currentLocale}/premium`}>
                  <span className="rounded-md bg-accent/20 px-2 py-0.5 text-xs font-extrabold text-foreground hover:underline">
                    {t("upgradeVip")}
                  </span>
                </Link>
              )}
            </div>
          </div>
        </div>

        {/* Scrollable middle — trial + nav always reachable */}
        <div className="mt-2.5 min-h-0 flex-1 space-y-2.5 overflow-y-auto overscroll-contain pr-0.5 [scrollbar-gutter:stable]">
          <TrialCountdown isVip={isVip} validUntil={entitlement?.valid_until} loading={loading} compact />

          <nav className="space-y-1">
            {primaryLinks.map(renderLink)}

            <div className="pt-1">
              <button
                type="button"
                aria-expanded={showMore}
                onClick={() => setMoreOpen((v) => !v)}
                className="sidebar-link sidebar-link-inactive w-full justify-between text-xs font-extrabold uppercase tracking-wider focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
              >
                {t("navMore")}
                <ChevronDown
                  aria-hidden="true"
                  className={`h-4 w-4 transition-transform ${showMore ? "rotate-180" : ""}`}
                />
              </button>
              {showMore && <div className="mt-1 space-y-1">{moreLinks.map(renderLink)}</div>}
            </div>
          </nav>
        </div>

        {/* Footer — pinned */}
        <div className="mt-2 shrink-0 space-y-2 border-t border-border pt-2.5">
          <div className="flex items-center justify-between px-0.5">
            <LocaleSwitcher size="md" className="border-border bg-background shadow-raised-sm" />
            <ThemeToggle />
          </div>

          <Link href={`/${currentLocale}/profile`}>
            <div className="sidebar-panel flex items-center gap-2.5 p-2.5 transition-[border-color,transform,box-shadow] hover:border-accent surface-interactive">
              <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-accent/20 text-sm font-black text-foreground shadow-raised-sm">
                {userName.charAt(0).toUpperCase()}
              </div>
              <div className="flex min-w-0 flex-col truncate">
                <span className="truncate text-sm font-bold text-foreground">{userName}</span>
                <span className="text-xs text-muted-foreground">{t("viewProfile")}</span>
              </div>
            </div>
          </Link>
        </div>
      </aside>
    </>
  );
}
