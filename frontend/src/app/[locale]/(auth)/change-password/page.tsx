"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { useLocale, useTranslations } from "next-intl";
import { useRouter } from "next/navigation";
import { BrandLogo } from "@/components/brand/brand-logo";
import { ThemeToggle } from "@/components/theme-toggle";
import { ChangePasswordForm } from "@/components/profile/change-password-form";
import { ApiError, apiGet } from "@/lib/api-client";
import { Button } from "@/components/ui/button";
import { ShieldCheck } from "lucide-react";
import { postLogoutPath } from "@/lib/telegram/logout";
import { isTelegramMiniApp } from "@/lib/telegram/web-app";

type MeResponse = {
  profile: { must_change_password?: boolean };
};

export default function ChangePasswordPage() {
  const t = useTranslations("Profile");
  const locale = useLocale();
  const router = useRouter();
  const [checking, setChecking] = useState(true);
  const [loadFailed, setLoadFailed] = useState(false);
  const [attempt, setAttempt] = useState(0);
  const retry = useCallback(() => {
    setLoadFailed(false);
    setChecking(true);
    setAttempt((n) => n + 1);
  }, []);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        await apiGet<MeResponse>("me");
      } catch (err) {
        if (cancelled) return;
        // In the Mini App /tg signs the learner straight back in and lands
        // here again, so only a dead session (401) may go there; a degraded
        // backend would otherwise loop between the two screens.
        if (isTelegramMiniApp() && !(err instanceof ApiError && err.status === 401)) {
          setLoadFailed(true);
        } else {
          router.replace(postLogoutPath(locale, `/${locale}/login`));
          return;
        }
      } finally {
        if (!cancelled) setChecking(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [attempt, locale, router]);

  async function handleSuccess() {
    try {
      await fetch("/api/auth/logout", { method: "POST" });
    } catch {
      /* best-effort */
    }
    // In the Mini App /tg signs the learner back in with the launch data —
    // the forced change is done, so there is no reason to type the new
    // password again. Not a deliberate logout: auto-login stays on.
    router.replace(postLogoutPath(locale, `/${locale}/login`));
  }

  return (
    <div
      className="asphalt-hero flex min-h-screen flex-col bg-background auth-safe-bottom"
    >
      <header
        className="flex h-14 items-center justify-between border-b border-border px-3 sm:px-4 auth-safe-top"
      >
        <Link
          href={`/${locale}`}
          className="flex min-w-0 items-center gap-2 font-display text-lg font-black text-foreground sm:gap-2.5 sm:text-xl"
        >
          <BrandLogo size={36} className="h-8 w-8 shrink-0 rounded-2xl object-cover sm:h-9 sm:w-9" />
          <span className="truncate">{t("passwordForcedBrand")}</span>
        </Link>
        <ThemeToggle />
      </header>

      <main className="flex flex-1 items-center justify-center p-3 sm:p-4">
        <div className="w-full max-w-sm animate-fade-in space-y-5 rounded-2xl border border-border bg-card p-5 sm:space-y-6 sm:p-8">
          <div className="space-y-2">
            <h1 className="font-display text-2xl font-extrabold tracking-tight">{t("passwordForcedTitle")}</h1>
            <p className="text-sm text-muted-foreground">{t("passwordForcedSubtitle")}</p>
          </div>

          {checking ? (
            <p role="status" className="text-sm text-muted-foreground">{t("loading")}</p>
          ) : loadFailed ? (
            <div role="alert" className="space-y-3 rounded-xl border border-destructive/50 bg-destructive/10 p-3 text-sm text-destructive">
              <p>{t("loadError")}</p>
              <Button type="button" variant="outline" size="sm" className="min-h-11" onClick={retry}>
                {t("retry")}
              </Button>
            </div>
          ) : (
            <ChangePasswordForm bare onSuccess={() => void handleSuccess()} />
          )}

          <div className="flex items-center justify-center gap-1.5 text-[11px] text-muted-foreground">
            <ShieldCheck aria-hidden="true" className="h-3.5 w-3.5 text-success" />
            <span>{t("passwordForcedSecureNote")}</span>
          </div>
        </div>
      </main>
    </div>
  );
}
