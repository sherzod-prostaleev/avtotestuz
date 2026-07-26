"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { useLocale, useTranslations } from "next-intl";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import { BrandLogo } from "@/components/brand/brand-logo";
import { ThemeToggle } from "@/components/theme-toggle";
import { ArrowLeft, Lock, Phone, ShieldCheck } from "lucide-react";
import { applyPendingReferralCode, capturePendingReferralCodeFromUrl } from "@/lib/referral-storage";
import { migrateDemoProgressOnLogin } from "@/lib/demo-progress-storage";

const ERROR_MESSAGE_KEYS: Record<string, string> = {
  invalid_phone: "errorInvalidPhone",
  invalid_credentials: "errorInvalidCredentials",
  account_blocked: "errorAccountBlocked",
  password_not_set: "errorPasswordNotSet",
  rate_limited: "errorRateLimited",
  network_error: "errorNetwork",
  weak_password: "errorWeakPassword",
};

function normalizePhone(input: string): string {
  return input.replace(/\D/g, "");
}

export default function LoginPage() {
  const t = useTranslations("Login");
  const locale = useLocale();
  const router = useRouter();
  const [phone, setPhone] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [setPasswordMode, setSetPasswordMode] = useState(false);

  useEffect(() => {
    capturePendingReferralCodeFromUrl();
  }, []);

  async function finishAuth() {
    // Side-effects must never block a successful login — cookies are already set.
    try {
      await applyPendingReferralCode();
    } catch {
      /* best-effort */
    }
    try {
      await migrateDemoProgressOnLogin();
    } catch {
      /* best-effort */
    }
    router.push(`/${locale}/dashboard`);
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      const endpoint = setPasswordMode ? "/api/auth/set-password" : "/api/auth/login";
      let res: Response;
      try {
        res = await fetch(endpoint, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ phone: normalizePhone(phone), password }),
        });
      } catch {
        setError("network_error");
        return;
      }

      let code = "unknown";
      try {
        const json = (await res.json()) as { error?: { code?: string } };
        code = json.error?.code ?? "unknown";
      } catch {
        if (!res.ok) {
          setError("network_error");
          return;
        }
      }

      if (!res.ok) {
        if (code === "password_not_set") {
          setSetPasswordMode(true);
        }
        setError(code === "unknown" && res.status >= 500 ? "network_error" : code);
        return;
      }
      await finishAuth();
    } finally {
      setSubmitting(false);
    }
  }

  const canSubmit = phone.length === 9 && password.length >= 8 && !submitting;

  return (
    <div className="asphalt-hero flex min-h-screen flex-col bg-background">
      <header className="flex h-14 items-center justify-between border-b border-border px-4">
        <Link
          href={`/${locale}`}
          className="flex items-center gap-2.5 font-display text-xl font-black text-foreground"
        >
          <BrandLogo size={36} className="h-9 w-9 rounded-2xl object-cover" />
          <span>{t("brandName")}</span>
        </Link>
        <ThemeToggle />
      </header>

      <main className="flex flex-1 items-center justify-center p-4">
        <div className="w-full max-w-sm animate-fade-in space-y-6 rounded-2xl border border-border bg-card p-8">
          <Link
            href={`/${locale}`}
            className="inline-flex min-h-11 items-center gap-1 text-xs font-semibold text-muted-foreground hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          >
            <ArrowLeft aria-hidden="true" className="h-3.5 w-3.5" /> {t("backHome")}
          </Link>

          <div className="space-y-2">
            <h1 className="font-display text-2xl font-extrabold tracking-tight">
              {setPasswordMode ? t("setPasswordTitle") : t("title")}
            </h1>
            <p className="text-sm text-muted-foreground">
              {setPasswordMode ? t("setPasswordSubtitle") : t("subtitle")}
            </p>
          </div>

          <form onSubmit={handleSubmit} className="space-y-4">
            <div className="space-y-1.5">
              <label
                htmlFor="login-phone"
                className="text-[11px] font-extrabold uppercase tracking-wider text-muted-foreground"
              >
                {t("phoneLabel")}
              </label>
              <div className="flex items-center gap-2 rounded-2xl border border-border bg-background px-4 py-3 text-sm transition-colors focus-within:border-accent focus-within:ring-2 focus-within:ring-ring">
                <Phone aria-hidden="true" className="h-4 w-4 shrink-0 text-muted-foreground" />
                <span className="font-bold text-foreground">+998</span>
                <input
                  id="login-phone"
                  type="tel"
                  inputMode="numeric"
                  autoComplete="tel-national"
                  value={phone}
                  onChange={(e) => setPhone(normalizePhone(e.target.value).slice(0, 9))}
                  placeholder="90 123 45 67"
                  className="w-full bg-transparent font-bold tracking-wide outline-none placeholder:font-normal placeholder:text-muted-foreground"
                  aria-label={t("phoneLabel")}
                />
              </div>
            </div>

            <div className="space-y-1.5">
              <label
                htmlFor="login-password"
                className="text-[11px] font-extrabold uppercase tracking-wider text-muted-foreground"
              >
                {t("passwordLabel")}
              </label>
              <div className="flex items-center gap-2 rounded-2xl border border-border bg-background px-4 py-3 text-sm transition-colors focus-within:border-accent focus-within:ring-2 focus-within:ring-ring">
                <Lock aria-hidden="true" className="h-4 w-4 shrink-0 text-muted-foreground" />
                <input
                  id="login-password"
                  type="password"
                  autoComplete={setPasswordMode ? "new-password" : "current-password"}
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  placeholder={t("passwordPlaceholder")}
                  className="w-full bg-transparent font-bold tracking-wide outline-none placeholder:font-normal placeholder:text-muted-foreground"
                  aria-label={t("passwordLabel")}
                />
              </div>
            </div>

            {error && (
              <div
                role="alert"
                className="rounded-xl border border-danger/50 bg-danger/10 p-3 text-xs font-semibold text-danger"
              >
                {t(ERROR_MESSAGE_KEYS[error] ?? "errorUnknown")}
              </div>
            )}

            <Button
              type="submit"
              variant="game"
              size="lg"
              className="w-full py-3 text-sm font-extrabold"
              disabled={!canSubmit}
            >
              {submitting
                ? t("submitting")
                : setPasswordMode
                  ? t("setPasswordSubmit")
                  : t("submit")}
            </Button>
          </form>

          {!setPasswordMode && (
            <p className="text-center text-xs text-muted-foreground">
              {t("noAccount")}{" "}
              <Link
                href={`/${locale}/register`}
                className="font-bold text-foreground underline-offset-2 hover:underline"
              >
                {t("registerLink")}
              </Link>
            </p>
          )}

          <div className="flex items-center justify-center gap-1.5 text-[11px] text-muted-foreground">
            <ShieldCheck aria-hidden="true" className="h-3.5 w-3.5 text-success" />
            <span>{t("secureNote")}</span>
          </div>
        </div>
      </main>
    </div>
  );
}
