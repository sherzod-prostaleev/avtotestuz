"use client";

import { useLocale } from "next-intl";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { ThemeToggle } from "@/components/theme-toggle";
import { useUserStats } from "@/hooks/use-user-stats";
import { Flame, Crown, User, BookOpen, Signpost, Target, AlertTriangle, LayoutDashboard } from "lucide-react";

export function Header() {
  const currentLocale = useLocale();
  const pathname = usePathname();
  const router = useRouter();
  const { streak, entitlement } = useUserStats();

  const isVip = entitlement?.is_vip ?? false;
  const currentStreak = streak?.current_streak ?? 0;

  const handleLanguageChange = (newLocale: string) => {
    if (newLocale === currentLocale) return;
    const newPath = pathname.replace(`/${currentLocale}`, `/${newLocale}`);
    router.push(newPath);
  };

  const navLinks = [
    { href: `/${currentLocale}/dashboard`, label: "Bosh sahifa", icon: LayoutDashboard },
    { href: `/${currentLocale}/tickets`, label: "Biletlar", icon: BookOpen },
    { href: `/${currentLocale}/signs`, label: "Yo'l belgilari", icon: Signpost },
    { href: `/${currentLocale}/practice`, label: "Mashq", icon: Target },
    { href: `/${currentLocale}/mistakes`, label: "Xatolar", icon: AlertTriangle },
  ];

  return (
    <header className="sticky top-0 z-40 w-full border-b border-border/80 bg-background/80 backdrop-blur-md">
      <div className="mx-auto flex h-16 max-w-6xl items-center justify-between px-4">
        {/* Brand Logo */}
        <Link href={`/${currentLocale}/dashboard`} className="flex items-center gap-2 font-display text-xl font-extrabold tracking-tight">
          <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-accent text-white shadow-3d transition-transform hover:scale-105">
            🚗
          </div>
          <span className="bg-gradient-to-r from-accent to-indigo-500 bg-clip-text text-transparent">
            AvtoTest
          </span>
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
                className={`flex items-center gap-1.5 rounded-lg px-3 py-2 text-xs font-bold transition-all ${
                  isActive
                    ? "bg-accent/15 text-accent"
                    : "text-muted-foreground hover:bg-card hover:text-foreground"
                }`}
              >
                <Icon className="h-4 w-4" />
                {link.label}
              </Link>
            );
          })}
        </nav>

        {/* Right Action Icons & Profile */}
        <div className="flex items-center gap-3">
          {/* Streak Flame Pill */}
          <div className="flex items-center gap-1.5 rounded-full border border-streak/30 bg-streak/10 px-3 py-1 text-xs font-extrabold text-streak">
            <Flame className="h-4 w-4 animate-flame text-streak" />
            <span>{currentStreak}</span>
          </div>

          {/* VIP Badge */}
          <Link href={`/${currentLocale}/premium`}>
            <div
              className={`flex items-center gap-1 rounded-full px-2.5 py-1 text-xs font-bold transition-transform hover:scale-105 ${
                isVip
                  ? "bg-gold/20 text-gold border border-gold/40"
                  : "bg-card border border-border text-muted-foreground hover:border-gold/50"
              }`}
            >
              <Crown className={`h-3.5 w-3.5 ${isVip ? "text-gold" : ""}`} />
              <span className="hidden sm:inline">{isVip ? "VIP" : "Bepul"}</span>
            </div>
          </Link>

          {/* Language Switcher */}
          <div className="flex gap-0.5 rounded-lg border border-border/80 bg-card p-0.5">
            {[
              { code: "uz-Latn", label: "O'z" },
              { code: "uz-Cyrl", label: "Ўз" },
              { code: "ru", label: "Ru" },
            ].map((lang) => (
              <button
                key={lang.code}
                onClick={() => handleLanguageChange(lang.code)}
                className={`rounded px-2 py-0.5 text-[11px] font-bold transition-all ${
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

          {/* Profile Pill */}
          <Link href={`/${currentLocale}/profile`}>
            <div className="flex h-9 w-9 items-center justify-center rounded-full border border-border bg-card text-muted-foreground transition-all hover:border-accent hover:text-accent">
              <User className="h-4 w-4" />
            </div>
          </Link>
        </div>
      </div>
    </header>
  );
}
