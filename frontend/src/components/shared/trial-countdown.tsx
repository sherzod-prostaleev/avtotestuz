"use client";

import { useEffect, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import Link from "next/link";
import { Crown, Timer } from "lucide-react";

interface TrialCountdownProps {
  isVip: boolean;
  validUntil?: string | null;
  /**
   * True while the entitlement is still being fetched. We intentionally
   * render nothing while loading: a free user used to stay empty→empty
   * (stable), and showing a skeleton here made that majority path flash a
   * box that then vanished. VIP users still see the panel appear once the
   * fetch resolves — appearing is less jarring than the F5 vanish this
   * change originally targeted. The VIP/free *badge* in the sidebar is
   * what reserves layout during loading.
   */
  loading?: boolean;
  /** Tighter padding for the sidebar so nav items stay visible. */
  compact?: boolean;
}

function remainingMs(validUntil: string): number {
  const end = Date.parse(validUntil);
  if (Number.isNaN(end)) return 0;
  return Math.max(0, end - Date.now());
}

function formatRemaining(ms: number): string {
  const total = Math.floor(ms / 1000);
  const hours = Math.floor(total / 3600);
  const minutes = Math.floor((total % 3600) / 60);
  const seconds = total % 60;
  return [hours, minutes, seconds].map((part) => part.toString().padStart(2, "0")).join(":");
}

/**
 * Countdown for a time-boxed entitlement. Unlike the exam timer this derives
 * the remaining time from the expiry timestamp on every tick rather than
 * decrementing a local counter, so a suspended tab resumes showing the truth
 * instead of however long it managed to tick.
 *
 * The clock text is client-only after mount so SSR/hydration never disagree
 * on Date.now() by a second.
 */
export function TrialCountdown({
  isVip,
  validUntil,
  loading = false,
  compact = false,
}: TrialCountdownProps) {
  const t = useTranslations("Trial");
  const locale = useLocale();
  const [mounted, setMounted] = useState(false);
  const [remaining, setRemaining] = useState(0);

  useEffect(() => {
    setMounted(true);
  }, []);

  useEffect(() => {
    if (!validUntil) {
      setRemaining(0);
      return;
    }
    setRemaining(remainingMs(validUntil));
    const id = setInterval(() => setRemaining(remainingMs(validUntil)), 1000);
    return () => clearInterval(id);
  }, [validUntil]);

  // Prefer free-user layout stability over reserving a VIP-only slot we
  // usually leave empty. See prop docs above.
  if (loading || !isVip || !validUntil) return null;

  const shell = compact
    ? "rounded-xl border border-gold/40 bg-gold/10 p-2.5"
    : "rounded-2xl border border-gold/40 bg-gold/10 p-3.5";

  // Same shell on server + first client paint; fill the clock after mount.
  if (!mounted) {
    return (
      <div className={shell} aria-hidden="true">
        <div className="h-3 w-24 rounded bg-gold/20" />
        <div className={`mt-1.5 rounded bg-gold/15 ${compact ? "h-6 w-24" : "h-7 w-28"}`} />
        {!compact && <div className="mt-2 h-3 w-20 rounded bg-gold/10" />}
      </div>
    );
  }

  if (remaining <= 0) {
    return (
      <div className={`${shell} text-center`}>
        <p className="font-display text-sm font-extrabold text-gold">{t("expiredTitle")}</p>
        <p className="mt-1 text-xs leading-relaxed text-muted-foreground">{t("expiredBody")}</p>
        <Link
          href={`/${locale}/premium`}
          className="mt-2 inline-flex min-h-10 items-center justify-center gap-1.5 rounded-xl bg-gold px-3 text-xs font-extrabold text-slate-950 transition-colors hover:brightness-105 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        >
          <Crown aria-hidden="true" className="h-4 w-4" />
          {t("upgrade")}
        </Link>
      </div>
    );
  }

  return (
    <Link
      href={`/${locale}/premium`}
      className={`block transition-colors hover:bg-gold/20 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring ${shell}`}
    >
      <div className="flex items-center gap-1.5 text-xs font-extrabold uppercase tracking-wider text-gold">
        <Timer aria-hidden="true" className="h-3.5 w-3.5" />
        {t("label")}
      </div>
      <p
        data-testid="trial-countdown"
        className={`mt-0.5 font-display font-black tabular-nums text-gold ${
          compact ? "text-xl leading-none" : "text-2xl"
        }`}
        suppressHydrationWarning
      >
        {formatRemaining(remaining)}
      </p>
      {!compact && <p className="text-xs text-muted-foreground">{t("remaining")}</p>}
    </Link>
  );
}
