"use client";

import { useEffect, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { BrandLogo } from "@/components/brand/brand-logo";
import { LocaleSwitcher } from "@/components/locale-switcher";
import { ThemeToggle } from "@/components/theme-toggle";
import { TrialCountdown } from "@/components/shared/trial-countdown";
import { NotificationBell } from "@/components/notifications/notification-bell";
import { useUserStats } from "@/hooks/use-user-stats";
import { useVariantCount } from "@/hooks/use-variant-count";
import { getMySupportUnread } from "@/lib/support-chat-client";
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
  LifeBuoy,
} from "lucide-react";

type NavLink = {
  href: string;
  label: string;
  icon: typeof LayoutDashboard;
  isGold?: boolean;
  badge?: number;
};

export function Sidebar() {
  const currentLocale = useLocale();
  const t = useTranslations("Sidebar");
  const pathname = usePathname();
  const [mobileOpen, setMobileOpen] = useState(false);
  const [supportUnread, setSupportUnread] = useState(0);

  const { streak, entitlement, user, loading } = useUserStats();
  const ticketCount = useVariantCount();

  const isVip = entitlement?.is_vip ?? false;
  const currentStreak = streak?.current_streak ?? 0;
  const userName = user?.name || t("userFallback");

  useEffect(() => {
    let cancelled = false;
    getMySupportUnread()
      .then((r) => {
        if (!cancelled) setSupportUnread(r.unread ?? 0);
      })
      .catch(() => {
        /* ignore — badge is best-effort */
      });
    return () => {
      cancelled = true;
    };
  }, [pathname]);

  const primaryLinks: NavLink[] = [
    { href: `/${currentLocale}/dashboard`, label: t("navDashboard"), icon: LayoutDashboard },
    { href: `/${currentLocale}/tickets`, label: t("navTickets", { count: ticketCount }), icon: BookOpen },
    { href: `/${currentLocale}/practice`, label: t("navPractice"), icon: Target },
    { href: `/${currentLocale}/arena`, label: t("navArena"), icon: Swords },
    { href: `/${currentLocale}/session/start?mode=exam`, label: t("navExam"), icon: Award },
    { href: `/${currentLocale}/signs`, label: t("navSigns"), icon: Signpost },
    { href: `/${currentLocale}/premium`, label: t("navPremium"), icon: Crown, isGold: true },
    {
      href: `/${currentLocale}/support`,
      label: t("navSupport"),
      icon: LifeBuoy,
      badge: supportUnread,
    },
    { href: `/${currentLocale}/profile`, label: t("navProfile"), icon: User },
  ];

  // Bottom tabs already cover home / tickets / practice / exam — drawer keeps the rest.
  const mobileDrawerLinks: NavLink[] = [
    { href: `/${currentLocale}/arena`, label: t("navArena"), icon: Swords },
    { href: `/${currentLocale}/signs`, label: t("navSigns"), icon: Signpost },
    { href: `/${currentLocale}/premium`, label: t("navPremium"), icon: Crown, isGold: true },
    {
      href: `/${currentLocale}/support`,
      label: t("navSupport"),
      icon: LifeBuoy,
      badge: supportUnread,
    },
    { href: `/${currentLocale}/profile`, label: t("navProfile"), icon: User },
  ];

  const moreLinks: NavLink[] = [
    { href: `/${currentLocale}/mistakes`, label: t("navMistakes"), icon: AlertTriangle },
    { href: `/${currentLocale}/saved`, label: t("navSaved"), icon: Bookmark },
    { href: `/${currentLocale}/leaderboard`, label: t("navLeaderboard"), icon: Trophy },
    { href: `/${currentLocale}/stats`, label: t("navStats"), icon: BarChart3 },
  ];

  const bottomTabs = [
    {
      href: `/${currentLocale}/dashboard`,
      label: t("navTabHome"),
      icon: LayoutDashboard,
      match: (path: string) => path.includes("/dashboard"),
    },
    {
      href: `/${currentLocale}/tickets`,
      label: t("navTabTickets"),
      icon: BookOpen,
      match: (path: string) => path.includes("/tickets"),
    },
    {
      href: `/${currentLocale}/practice`,
      label: t("navTabPractice"),
      icon: Target,
      match: (path: string) => path.includes("/practice"),
    },
    {
      href: `/${currentLocale}/session/start?mode=exam`,
      label: t("navTabExam"),
      icon: Award,
      match: (path: string) => path.includes("/session"),
    },
  ] as const;

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
              ? "border border-gold/30 bg-gold/10 text-gold hover:bg-gold/15"
              : "sidebar-link-inactive"
        }`}
      >
        <Icon aria-hidden="true" className="h-3.5 w-3.5 shrink-0 opacity-90" />
        <span className="min-w-0 flex-1 leading-snug">{link.label}</span>
        {link.badge && link.badge > 0 ? (
          <span className="ml-auto inline-flex min-w-[1.25rem] items-center justify-center rounded-full bg-destructive px-1.5 text-[10px] font-bold text-destructive-foreground">
            {link.badge > 99 ? "99+" : link.badge}
          </span>
        ) : null}
      </Link>
    );
  };

  const moreSection = (
    <div className="pt-0.5 md:pt-1.5">
      <button
        type="button"
        aria-expanded={showMore}
        onClick={() => setMoreOpen((v) => !v)}
        className="sidebar-link sidebar-link-inactive w-full justify-between text-[11px] font-extrabold uppercase tracking-wider focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
      >
        {t("navMore")}
        <ChevronDown
          aria-hidden="true"
          className={`h-3.5 w-3.5 transition-transform ${showMore ? "rotate-180" : ""}`}
        />
      </button>
      {showMore && <div className="mt-0.5 space-y-0.5 md:mt-1.5 md:space-y-1.5">{moreLinks.map(renderLink)}</div>}
    </div>
  );

  return (
    <>
      {/* Compact top chrome — brand + streak; full nav lives in bottom tabs + drawer */}
      <div
        className="sticky top-0 z-40 flex w-full items-center justify-between gap-2 border-b border-border bg-background/95 px-3 backdrop-blur-md md:hidden"
        style={{
          paddingTop: "max(0.5rem, env(safe-area-inset-top))",
          minHeight: "3.5rem",
        }}
      >
        <Link
          href={`/${currentLocale}/dashboard`}
          className="flex min-h-11 min-w-0 items-center gap-2 font-display text-lg font-bold text-foreground"
        >
          <BrandLogo size={32} className="h-8 w-8 shrink-0 rounded-2xl object-cover" />
          <span className="truncate font-black">{t("brandName")}</span>
        </Link>

        <div className="flex shrink-0 items-center gap-1.5">
          <NotificationBell variant="mobile" />
          <div className="flex min-h-10 items-center gap-1 rounded-xl border border-streak/30 bg-streak/10 px-2.5 text-sm font-bold text-streak">
            <Flame aria-hidden="true" className="h-4 w-4 animate-flame" />
            <span className="tabular-nums">{currentStreak}</span>
          </div>

          <button
            type="button"
            aria-label={mobileOpen ? t("closeMenu") : t("openMenu")}
            onClick={() => setMobileOpen(!mobileOpen)}
            className="flex h-10 w-10 items-center justify-center rounded-xl border border-border bg-card text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
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
        className={`fixed bottom-0 left-0 top-0 z-50 flex w-[min(15.5rem,78vw)] flex-col overflow-hidden border-r border-border bg-card p-2.5 shadow-[6px_0_28px_-18px_hsl(var(--elev-ambient)/0.65)] transition-transform duration-300 md:w-64 md:translate-x-0 md:p-3 ${
          mobileOpen ? "translate-x-0" : "-translate-x-full md:translate-x-0"
        }`}
        style={{
          paddingTop: "max(0.5rem, env(safe-area-inset-top))",
          paddingBottom: "max(0.5rem, env(safe-area-inset-bottom))",
        }}
      >
        <div className="shrink-0 space-y-1.5 md:space-y-2.5">
          <div className="flex items-center justify-between gap-1 px-0.5">
            <Link
              href={`/${currentLocale}/dashboard`}
              onClick={() => setMobileOpen(false)}
              className="flex min-w-0 items-center gap-1.5 font-display text-base font-black text-foreground md:gap-2 md:text-lg"
            >
              <BrandLogo
                size={28}
                className="h-7 w-7 shrink-0 rounded-xl object-cover md:h-9 md:w-9 md:rounded-2xl"
              />
              <span className="truncate">{t("brandName")}</span>
            </Link>

            <div className="flex items-center gap-1">
              <div className="hidden md:block">
                <NotificationBell variant="desktop" />
              </div>
              <button
                type="button"
                aria-label={t("closeMenu")}
                onClick={() => setMobileOpen(false)}
                className="flex h-9 w-9 items-center justify-center rounded-lg text-muted-foreground hover:bg-background hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring md:hidden"
              >
                <X aria-hidden="true" className="h-4 w-4" />
              </button>
            </div>
          </div>

          {/* Streak lives in the mobile top bar — keep the panel for desktop only. */}
          <div className="sidebar-panel hidden px-3 py-2 md:block">
            <div className="flex items-center justify-between gap-2 text-sm font-extrabold">
              <span className="flex min-w-0 items-center gap-1.5 truncate text-streak">
                <Flame aria-hidden="true" className="h-4 w-4 shrink-0 animate-flame" />
                <span className="truncate">{t("streakCount", { count: currentStreak })}</span>
              </span>
              {loading ? (
                <span aria-hidden="true" className="h-5 w-14 animate-pulse rounded-full bg-border/60" />
              ) : isVip ? (
                <span className="shrink-0 rounded-md border border-gold/30 bg-gold/15 px-2 py-0.5 text-xs font-extrabold text-gold">
                  {t("vipBadge")}
                </span>
              ) : (
                <Link href={`/${currentLocale}/premium`} className="shrink-0">
                  <span className="rounded-md bg-accent/20 px-2 py-0.5 text-xs font-extrabold text-foreground hover:underline">
                    {t("upgradeVip")}
                  </span>
                </Link>
              )}
            </div>
          </div>
        </div>

        <div className="mt-1.5 min-h-0 flex-1 space-y-1.5 overflow-y-auto overscroll-contain pr-0.5 [scrollbar-gutter:stable] md:mt-2.5 md:space-y-2.5">
          <TrialCountdown isVip={isVip} validUntil={entitlement?.valid_until} loading={loading} compact />

          <nav className="space-y-0.5 md:space-y-1.5">
            <div className="space-y-0.5 md:hidden">{mobileDrawerLinks.map(renderLink)}</div>
            <div className="hidden space-y-1.5 md:block">{primaryLinks.map(renderLink)}</div>
            {moreSection}
          </nav>
        </div>

        <div className="relative z-20 mt-1.5 shrink-0 border-t border-border pt-1.5 md:mt-2 md:pt-2.5">
          <div className="sidebar-panel overflow-hidden">
            <div className="flex items-center gap-1 border-b border-border/80 bg-muted/25 p-1 md:gap-1.5 md:p-1.5">
              <LocaleSwitcher size="sm" fill embedded className="min-w-0 flex-1 md:hidden" />
              <LocaleSwitcher size="md" fill embedded className="hidden min-w-0 flex-1 md:flex" />
              <ThemeToggle size="sm" embedded className="md:hidden" />
              <ThemeToggle size="md" embedded className="hidden md:inline-flex" />
            </div>

            <Link
              href={`/${currentLocale}/profile`}
              onClick={() => setMobileOpen(false)}
              className="flex items-center gap-2 p-2 transition-colors hover:bg-accent/10 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring md:gap-2.5 md:p-2.5"
            >
              <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-accent/20 text-xs font-black text-foreground shadow-raised-sm md:h-10 md:w-10 md:rounded-xl md:text-sm">
                {userName.charAt(0).toUpperCase()}
              </div>
              <div className="flex min-w-0 flex-col truncate">
                <span className="truncate text-sm font-bold text-foreground">{userName}</span>
                <span className="text-[11px] text-muted-foreground md:text-xs">{t("viewProfile")}</span>
              </div>
            </Link>
          </div>
        </div>
      </aside>

      {/* Thumb-zone primary destinations */}
      <nav className="app-bottom-nav" aria-label={t("brandName")}>
        <div className="flex items-stretch">
          {bottomTabs.map((tab) => {
            const Icon = tab.icon;
            const active = tab.match(pathname);
            return (
              <Link
                key={tab.href}
                href={tab.href}
                className={`app-bottom-nav-item ${active ? "app-bottom-nav-item-active" : ""}`}
              >
                <Icon aria-hidden="true" className="h-5 w-5" />
                <span className="truncate">{tab.label}</span>
              </Link>
            );
          })}
          <button
            type="button"
            aria-label={t("openMenu")}
            aria-pressed={mobileOpen}
            onClick={() => setMobileOpen(true)}
            className={`app-bottom-nav-item ${mobileOpen ? "app-bottom-nav-item-active" : ""}`}
          >
            <Menu aria-hidden="true" className="h-5 w-5" />
            <span>{t("navTabMore")}</span>
          </button>
        </div>
      </nav>
    </>
  );
}
