"use client";

import { useState } from "react";
import { useLocale } from "next-intl";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { ThemeToggle } from "@/components/theme-toggle";
import { useUserStats } from "@/hooks/use-user-stats";
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
  Sparkles,
} from "lucide-react";

export function Sidebar() {
  const currentLocale = useLocale();
  const pathname = usePathname();
  const router = useRouter();
  const [mobileOpen, setMobileOpen] = useState(false);

  const { streak, entitlement, user } = useUserStats();

  const isVip = entitlement?.is_vip ?? false;
  const currentStreak = streak?.current_streak ?? 0;
  const userName = user?.name || "O'quvchi";

  const handleLanguageChange = (newLocale: string) => {
    if (newLocale === currentLocale) return;
    const newPath = pathname.replace(`/${currentLocale}`, `/${newLocale}`);
    router.push(newPath);
  };

  const navLinks = [
    { href: `/${currentLocale}/dashboard`, label: "Bosh sahifa", icon: LayoutDashboard },
    { href: `/${currentLocale}/tickets`, label: "Biletlar (61 ta)", icon: BookOpen },
    { href: `/${currentLocale}/session/start?mode=exam`, label: "Imtihon simulyatsiyasi", icon: Award },
    { href: `/${currentLocale}/practice`, label: "Mashq rejimi", icon: Target },
    { href: `/${currentLocale}/signs`, label: "Yo'l belgilari", icon: Signpost },
    { href: `/${currentLocale}/mistakes`, label: "Xatolar banki", icon: AlertTriangle },
    { href: `/${currentLocale}/stats`, label: "Statistika va Tahlil", icon: BarChart3 },
    { href: `/${currentLocale}/profile`, label: "Profil va Sozlamalar", icon: User },
    { href: `/${currentLocale}/premium`, label: "Premium VIP", icon: Crown, isGold: true },
  ];

  return (
    <>
      {/* Mobile Top Header */}
      <div className="md:hidden sticky top-0 z-40 flex h-16 w-full items-center justify-between border-b border-border bg-background/90 px-4 backdrop-blur-md">
        <Link href={`/${currentLocale}/dashboard`} className="flex items-center gap-2 font-display text-lg font-bold">
          <div className="flex h-8 w-8 items-center justify-center rounded-xl bg-accent text-white shadow-3d">
            🚗
          </div>
          <span className="bg-gradient-to-r from-accent to-indigo-500 bg-clip-text text-transparent">
            AvtoTest
          </span>
        </Link>

        <div className="flex items-center gap-2">
          <div className="flex items-center gap-1 rounded-full border border-streak/30 bg-streak/10 px-2.5 py-1 text-xs font-bold text-streak">
            <Flame className="h-3.5 w-3.5 animate-flame" />
            <span>{currentStreak}</span>
          </div>

          <button
            onClick={() => setMobileOpen(!mobileOpen)}
            className="flex h-9 w-9 items-center justify-center rounded-xl border border-border bg-card text-foreground"
          >
            {mobileOpen ? <X className="h-5 w-5" /> : <Menu className="h-5 w-5" />}
          </button>
        </div>
      </div>

      {/* Backdrop Overlay (Mobile) */}
      {mobileOpen && (
        <div
          onClick={() => setMobileOpen(false)}
          className="md:hidden fixed inset-0 z-40 bg-black/60 backdrop-blur-sm"
        />
      )}

      {/* Sidebar Shell Container */}
      <aside
        className={`fixed top-0 bottom-0 left-0 z-50 flex w-64 flex-col justify-between border-r border-border/80 bg-card/95 p-4 shadow-xl backdrop-blur-xl transition-transform duration-300 md:translate-x-0 ${
          mobileOpen ? "translate-x-0" : "-translate-x-full md:translate-x-0"
        }`}
      >
        <div className="space-y-6">
          {/* Brand Logo Header */}
          <div className="flex items-center justify-between pt-2 px-2">
            <Link href={`/${currentLocale}/dashboard`} className="flex items-center gap-2.5 font-display text-xl font-black">
              <div className="flex h-10 w-10 items-center justify-center rounded-2xl bg-accent text-white shadow-3d hover:scale-105 transition-transform">
                🚗
              </div>
              <div className="flex flex-col">
                <span className="bg-gradient-to-r from-accent via-indigo-500 to-accent bg-clip-text text-transparent">
                  AvtoTest
                </span>
                <span className="text-[10px] font-bold text-muted-foreground uppercase tracking-widest">Pro Edition</span>
              </div>
            </Link>

            <button onClick={() => setMobileOpen(false)} className="md:hidden text-muted-foreground">
              <X className="h-5 w-5" />
            </button>
          </div>

          {/* Gamification Bar */}
          <div className="rounded-2xl border border-accent/20 bg-accent/5 p-3 space-y-2">
            <div className="flex items-center justify-between text-xs font-extrabold">
              <span className="flex items-center gap-1.5 text-streak">
                <Flame className="h-4 w-4 animate-flame" /> {currentStreak} Kunlik Streak
              </span>
              {isVip ? (
                <span className="rounded-full bg-gold/20 px-2 py-0.5 text-[10px] font-extrabold text-gold border border-gold/30">
                  VIP
                </span>
              ) : (
                <Link href={`/${currentLocale}/premium`}>
                  <span className="rounded-full bg-accent/20 px-2 py-0.5 text-[10px] font-extrabold text-accent hover:underline">
                    +VIP
                  </span>
                </Link>
              )}
            </div>
          </div>

          {/* Navigation Links List */}
          <nav className="space-y-1">
            {navLinks.map((link) => {
              const Icon = link.icon;
              const isActive = pathname === link.href || (link.href.includes("session") && pathname.includes("session"));

              return (
                <Link
                  key={link.href}
                  href={link.href}
                  onClick={() => setMobileOpen(false)}
                  className={`flex items-center justify-between rounded-xl px-3.5 py-2.5 text-xs font-bold transition-all ${
                    isActive
                      ? "bg-accent text-white shadow-3d translate-x-1"
                      : link.isGold
                      ? "text-gold hover:bg-gold/10"
                      : "text-muted-foreground hover:bg-background hover:text-foreground"
                  }`}
                >
                  <div className="flex items-center gap-3">
                    <Icon className={`h-4 w-4 ${link.isGold && !isActive ? "text-gold" : ""}`} />
                    <span>{link.label}</span>
                  </div>
                  {link.isGold && !isActive && <Sparkles className="h-3.5 w-3.5 text-gold animate-pulse" />}
                </Link>
              );
            })}
          </nav>
        </div>

        {/* Bottom Footer Controls (Locale, Theme & User) */}
        <div className="space-y-3 pt-4 border-t border-border">
          <div className="flex items-center justify-between px-1">
            {/* Language Switcher */}
            <div className="flex gap-0.5 rounded-lg border border-border bg-background p-0.5">
              {[
                { code: "uz-Latn", label: "O'z" },
                { code: "uz-Cyrl", label: "Ўз" },
                { code: "ru", label: "Ru" },
              ].map((lang) => (
                <button
                  key={lang.code}
                  onClick={() => handleLanguageChange(lang.code)}
                  className={`rounded px-2 py-0.5 text-[10px] font-bold transition-all ${
                    currentLocale === lang.code
                      ? "bg-accent text-white"
                      : "text-muted-foreground hover:text-foreground"
                  }`}
                >
                  {lang.label}
                </button>
              ))}
            </div>

            {/* Theme Toggle */}
            <ThemeToggle />
          </div>

          {/* User Profile Card Footer */}
          <Link href={`/${currentLocale}/profile`}>
            <div className="flex items-center gap-3 rounded-2xl border border-border bg-background/80 p-2.5 hover:border-accent transition-colors">
              <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-accent/15 text-accent font-black text-sm">
                {userName.charAt(0).toUpperCase()}
              </div>
              <div className="flex flex-col truncate">
                <span className="text-xs font-bold text-foreground truncate">{userName}</span>
                <span className="text-[10px] text-muted-foreground">Profilni ko'rish</span>
              </div>
            </div>
          </Link>
        </div>
      </aside>
    </>
  );
}
