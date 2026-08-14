"use client";

import { Suspense, useEffect, useMemo, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { Button } from "@/components/ui/button";
import { BrandLogo } from "@/components/brand/brand-logo";
import { ThemeToggle } from "@/components/theme-toggle";
import { LocaleSwitcher } from "@/components/locale-switcher";
import { ArrowLeft, CheckCircle2, ExternalLink, Lock } from "lucide-react";

const COMPLETE_ERROR_KEYS: Record<string, string> = {
  weak_password: "errorWeakPassword",
  reset_not_verified: "errorNotVerified",
  invalid_reset_token: "errorInvalidToken",
  rate_limited: "errorRateLimited",
  network_error: "errorNetwork",
};

export default function ResetPasswordPage() {
  return (
    <Suspense fallback={null}>
      <ResetPasswordInner />
    </Suspense>
  );
}

function ResetPasswordInner() {
  const t = useTranslations("PasswordReset");
  const loginT = useTranslations("Login");
  const locale = useLocale();
  const searchParams = useSearchParams();
  const token = (searchParams.get("token") ?? "").trim();
  const botURL = (searchParams.get("bot") ?? "").trim();
  const expSec = Math.min(900, Math.max(60, Number(searchParams.get("exp") ?? "900") || 900));

  const [state, setState] = useState<"pending" | "verified" | "invalid" | "success">(
    token ? "pending" : "invalid"
  );
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const telegramHref = useMemo(() => {
    if (!botURL) return null;
    try {
      const url = new URL(botURL);
      if (url.protocol !== "https:" || url.hostname !== "t.me") return null;
      return url.toString();
    } catch {
      return null;
    }
  }, [botURL]);

  useEffect(() => {
    if (!token || state === "success" || state === "invalid" || state === "verified") return;
    let stopped = false;
    const poll = async () => {
      try {
        const res = await fetch(`/api/auth/password-reset/status?token=${encodeURIComponent(token)}`);
        const json = (await res.json()) as { data?: { state?: string } };
        const next = json.data?.state;
        if (stopped) return;
        if (next === "verified") setState("verified");
        if (next === "invalid") setState("invalid");
      } catch {
        /* keep pending; next tick retries */
      }
    };
    void poll();
    const id = window.setInterval(() => void poll(), 2000);
    const expireId = window.setTimeout(() => {
      if (!stopped) setState((prev) => (prev === "pending" ? "invalid" : prev));
    }, expSec * 1000);
    return () => {
      stopped = true;
      window.clearInterval(id);
      window.clearTimeout(expireId);
    };
  }, [token, state, expSec]);

  async function handleComplete(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    if (password.length < 8) {
      setError("weak_password");
      return;
    }
    if (password !== confirm) {
      setError("password_mismatch");
      return;
    }
    setSubmitting(true);
    try {
      let res: Response;
      try {
        res = await fetch("/api/auth/password-reset/complete", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ token, password }),
        });
      } catch {
        setError("network_error");
        return;
      }
      let json: { error?: { code?: string } } = {};
      try {
        json = (await res.json()) as typeof json;
      } catch {
        if (!res.ok) {
          setError("network_error");
          return;
        }
      }
      if (!res.ok) {
        setError(json.error?.code ?? "unknown");
        return;
      }
      setState("success");
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

          {state === "success" ? (
            <div className="space-y-4 text-center">
              <CheckCircle2 aria-hidden="true" className="mx-auto h-12 w-12 text-success" />
              <h1 className="font-display text-2xl font-extrabold tracking-tight">{t("successTitle")}</h1>
              <p className="text-sm text-muted-foreground">{t("successBody")}</p>
              <Link href={`/${locale}/login`} className="block">
                <Button as="span" variant="game" size="lg" className="w-full">
                  {t("backToLogin")}
                </Button>
              </Link>
            </div>
          ) : state === "invalid" ? (
            <div className="space-y-4">
              <h1 className="font-display text-2xl font-extrabold tracking-tight">{t("title")}</h1>
              <p className="text-sm text-muted-foreground">{t("invalidLink")}</p>
              <Link href={`/${locale}/forgot-password`} className="block">
                <Button as="span" variant="game" size="lg" className="w-full">
                  {t("restart")}
                </Button>
              </Link>
            </div>
          ) : state === "pending" ? (
            <div className="space-y-4">
              <h1 className="font-display text-2xl font-extrabold tracking-tight">{t("waitingTitle")}</h1>
              <p className="text-sm text-muted-foreground">{t("waitingHint")}</p>
              {telegramHref ? (
                <a href={telegramHref} target="_blank" rel="noopener noreferrer" className="block">
                  <Button as="span" variant="game" size="lg" className="w-full">
                    <ExternalLink aria-hidden="true" className="mr-2 h-4 w-4" />
                    {t("openTelegram")}
                  </Button>
                </a>
              ) : (
                <Link href={`/${locale}/forgot-password`} className="block">
                  <Button as="span" variant="outline" size="lg" className="w-full">
                    {t("restart")}
                  </Button>
                </Link>
              )}
              <p role="status" className="text-center text-xs font-semibold text-muted-foreground">
                {t("waitingStatus")}
              </p>
            </div>
          ) : (
            <form onSubmit={handleComplete} className="space-y-4">
              <div className="space-y-2">
              <h1 className="font-display text-2xl font-extrabold tracking-tight">{t("title")}</h1>
              <p className="text-sm text-muted-foreground">{t("setPasswordHint")}</p>
              </div>
              <div className="space-y-1.5">
                <label htmlFor="new-password" className="text-[11px] font-extrabold uppercase tracking-wider text-muted-foreground">
                  {t("newPasswordLabel")}
                </label>
                <div className="flex items-center gap-2 rounded-2xl border border-border bg-background px-4 py-3 text-sm focus-within:border-accent focus-within:ring-2 focus-within:ring-ring">
                  <Lock aria-hidden="true" className="h-4 w-4 shrink-0 text-muted-foreground" />
                  <input
                    id="new-password"
                    type="password"
                    autoComplete="new-password"
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    placeholder={t("passwordPlaceholder")}
                    className="w-full bg-transparent font-bold outline-none"
                    aria-label={t("newPasswordLabel")}
                  />
                </div>
              </div>
              <div className="space-y-1.5">
                <label htmlFor="confirm-password" className="text-[11px] font-extrabold uppercase tracking-wider text-muted-foreground">
                  {t("confirmPasswordLabel")}
                </label>
                <div className="flex items-center gap-2 rounded-2xl border border-border bg-background px-4 py-3 text-sm focus-within:border-accent focus-within:ring-2 focus-within:ring-ring">
                  <Lock aria-hidden="true" className="h-4 w-4 shrink-0 text-muted-foreground" />
                  <input
                    id="confirm-password"
                    type="password"
                    autoComplete="new-password"
                    value={confirm}
                    onChange={(e) => setConfirm(e.target.value)}
                    placeholder={t("passwordPlaceholder")}
                    className="w-full bg-transparent font-bold outline-none"
                    aria-label={t("confirmPasswordLabel")}
                  />
                </div>
              </div>
              {error && (
                <div role="alert" className="rounded-xl border border-danger/50 bg-danger/10 p-3 text-xs font-semibold text-danger">
                  {error === "password_mismatch" ? t("errorPasswordMismatch") : t(COMPLETE_ERROR_KEYS[error] ?? "errorUnknown")}
                </div>
              )}
              <Button type="submit" variant="game" size="lg" className="w-full" disabled={submitting}>
                {submitting ? t("settingPassword") : t("setPassword")}
              </Button>
            </form>
          )}
        </div>
      </main>
    </div>
  );
}
