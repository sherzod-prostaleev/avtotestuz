"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useLocale, useTranslations } from "next-intl";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import { BrandLogo } from "@/components/brand/brand-logo";
import { ThemeToggle } from "@/components/theme-toggle";
import { LocaleSwitcher } from "@/components/locale-switcher";
import { ArrowLeft, Lock, Phone, Send } from "lucide-react";
import {
  formatNationalPhone,
  normalizeNationalPhone,
  parsePasswordResetTokenFromBotURL,
} from "@/lib/phone-format";

const ERROR_KEYS: Record<string, string> = {
  invalid_phone: "errorInvalidPhone",
  rate_limited: "errorRateLimited",
  network_error: "errorNetwork",
  telegram_bot_unconfigured: "errorTelegramUnconfigured",
};

export default function ForgotPasswordPage() {
  const t = useTranslations("PasswordReset");
  const loginT = useTranslations("Login");
  const locale = useLocale();
  const router = useRouter();
  const [phone, setPhone] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    const localPhone = normalizeNationalPhone(phone);
    if (localPhone.length !== 9) {
      setError("invalid_phone");
      return;
    }
    setSubmitting(true);
    try {
      let res: Response;
      try {
        res = await fetch("/api/auth/password-reset/start", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ phone: localPhone }),
        });
      } catch {
        setError("network_error");
        return;
      }
      let json: { error?: { code?: string }; data?: { bot_url?: string; expires_in_sec?: number } } = {};
      try {
        json = (await res.json()) as typeof json;
      } catch {
        setError("network_error");
        return;
      }
      if (!res.ok) {
        setError(json.error?.code ?? "unknown");
        return;
      }
      const token = json.data?.bot_url ? parsePasswordResetTokenFromBotURL(json.data.bot_url) : null;
      if (!token || !json.data?.bot_url) {
        setError("unknown");
        return;
      }
      const expires = Math.min(900, Math.max(60, json.data.expires_in_sec ?? 900));
      const params = new URLSearchParams({
        token,
        bot: json.data.bot_url,
        exp: String(expires),
      });
      router.push(`/${locale}/reset-password?${params.toString()}`);
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div
      className="asphalt-hero flex min-h-screen flex-col bg-background"
      style={{ paddingBottom: "env(safe-area-inset-bottom)" }}
    >
      <header
        className="flex h-14 items-center justify-between gap-2 border-b border-border px-3 sm:px-4"
        style={{ paddingTop: "env(safe-area-inset-top)" }}
      >
        <Link
          href={`/${locale}`}
          className="flex min-w-0 items-center gap-2 font-display text-lg font-black text-foreground sm:text-xl"
        >
          <BrandLogo size={36} className="h-8 w-8 shrink-0 rounded-2xl object-cover sm:h-9 sm:w-9" />
          <span className="truncate">{loginT("brandName")}</span>
        </Link>
        <div className="flex shrink-0 items-center gap-1.5">
          <LocaleSwitcher compact />
          <ThemeToggle />
        </div>
      </header>

      <main className="flex flex-1 items-center justify-center p-3 sm:p-4">
        <div className="w-full max-w-sm animate-fade-in space-y-5 rounded-2xl border border-border bg-card p-5 sm:space-y-6 sm:p-8">
          <Link
            href={`/${locale}/login`}
            className="inline-flex min-h-11 items-center gap-1 text-xs font-semibold text-muted-foreground hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          >
            <ArrowLeft aria-hidden="true" className="h-3.5 w-3.5" /> {t("backToLogin")}
          </Link>
          <div className="space-y-2">
            <h1 className="font-display text-2xl font-extrabold tracking-tight">{t("title")}</h1>
            <p className="text-sm text-muted-foreground">{t("subtitle")}</p>
          </div>
          <form onSubmit={handleSubmit} className="space-y-4">
            <div className="space-y-1.5">
              <label
                htmlFor="reset-phone"
                className="text-[11px] font-extrabold uppercase tracking-wider text-muted-foreground"
              >
                {t("phoneLabel")}
              </label>
              <div className="flex items-center gap-2 rounded-2xl border border-border bg-background px-4 py-3 text-sm transition-colors focus-within:border-accent focus-within:ring-2 focus-within:ring-ring">
                <Phone aria-hidden="true" className="h-4 w-4 shrink-0 text-muted-foreground" />
                <span className="font-bold text-foreground">+998</span>
                <input
                  id="reset-phone"
                  type="tel"
                  inputMode="numeric"
                  autoComplete="tel-national"
                  value={formatNationalPhone(phone)}
                  onChange={(e) => setPhone(normalizeNationalPhone(e.target.value))}
                  placeholder="90 123 45 67"
                  className="w-full bg-transparent font-bold tracking-wide outline-none placeholder:font-normal placeholder:text-muted-foreground"
                  aria-label={t("phoneLabel")}
                />
              </div>
            </div>
            {error && (
              <div role="alert" className="rounded-xl border border-danger/50 bg-danger/10 p-3 text-xs font-semibold text-danger">
                {t(ERROR_KEYS[error] ?? "errorUnknown")}
              </div>
            )}
            <Button type="submit" variant="game" size="lg" className="w-full py-3 text-sm font-extrabold" disabled={submitting}>
              <Send aria-hidden="true" className="mr-2 h-4 w-4" />
              {submitting ? t("submitting") : t("submit")}
            </Button>
          </form>
          <p className="flex items-center justify-center gap-1.5 text-[11px] text-muted-foreground">
            <Lock aria-hidden="true" className="h-3.5 w-3.5" />
            {loginT("secureNote")}
          </p>
        </div>
      </main>
    </div>
  );
}
