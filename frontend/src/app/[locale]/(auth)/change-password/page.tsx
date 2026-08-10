"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useLocale, useTranslations } from "next-intl";
import { useRouter } from "next/navigation";
import { BrandLogo } from "@/components/brand/brand-logo";
import { ThemeToggle } from "@/components/theme-toggle";
import { ChangePasswordForm } from "@/components/profile/change-password-form";
import { apiGet } from "@/lib/api-client";
import { ShieldCheck } from "lucide-react";

type MeResponse = {
  profile: { must_change_password?: boolean };
};

export default function ChangePasswordPage() {
  const t = useTranslations("Profile");
  const locale = useLocale();
  const router = useRouter();
  const [checking, setChecking] = useState(true);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        await apiGet<MeResponse>("me");
      } catch {
        if (!cancelled) {
          router.replace(`/${locale}/login`);
          return;
        }
      } finally {
        if (!cancelled) setChecking(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [locale, router]);

  async function handleSuccess() {
    try {
      await fetch("/api/auth/logout", { method: "POST" });
    } catch {
      /* best-effort */
    }
    router.replace(`/${locale}/login`);
  }

  return (
    <div
      className="asphalt-hero flex min-h-screen flex-col bg-background"
      style={{ paddingBottom: "env(safe-area-inset-bottom)" }}
    >
      <header
        className="flex h-14 items-center justify-between border-b border-border px-3 sm:px-4"
        style={{ paddingTop: "env(safe-area-inset-top)" }}
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
            <p className="text-sm text-muted-foreground">{t("loading")}</p>
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
