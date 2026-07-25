"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { useLocale, useTranslations } from "next-intl";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import { ThemeToggle } from "@/components/theme-toggle";
import { ArrowLeft, Lock, Phone, User } from "lucide-react";
import { applyPendingReferralCode, capturePendingReferralCodeFromUrl } from "@/lib/referral-storage";
import { migrateDemoProgressOnLogin } from "@/lib/demo-progress-storage";

const ERROR_MESSAGE_KEYS: Record<string, string> = {
  invalid_phone: "errorInvalidPhone",
  phone_taken: "errorPhoneTaken",
  weak_password: "errorWeakPassword",
  password_mismatch: "errorPasswordMismatch",
  rate_limited: "errorRateLimited",
  network_error: "errorNetwork",
};

function normalizePhone(input: string): string {
  return input.replace(/\D/g, "");
}

export default function RegisterPage() {
  const t = useTranslations("Register");
  const loginT = useTranslations("Login");
  const locale = useLocale();
  const router = useRouter();
  const [phone, setPhone] = useState("");
  const [name, setName] = useState("");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    capturePendingReferralCodeFromUrl();
  }, []);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    if (password !== confirmPassword) {
      setError("password_mismatch");
      return;
    }
    if (password.length < 8) {
      setError("weak_password");
      return;
    }
    setSubmitting(true);
    try {
      const res = await fetch("/api/auth/register", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          phone: normalizePhone(phone),
          password,
          name: name.trim() || undefined,
        }),
      });
      const json = await res.json();
      if (!res.ok) {
        setError(json.error?.code ?? "unknown");
        return;
      }
      await applyPendingReferralCode();
      await migrateDemoProgressOnLogin();
      router.push(`/${locale}/dashboard`);
    } catch {
      setError("network_error");
    } finally {
      setSubmitting(false);
    }
  }

  const canSubmit =
    phone.length === 9 && password.length >= 8 && confirmPassword.length >= 8 && !submitting;

  return (
    <div className="asphalt-hero flex min-h-screen flex-col bg-background">
      <header className="flex h-14 items-center justify-between border-b border-border px-4">
        <Link
          href={`/${locale}`}
          className="flex items-center gap-2.5 font-display text-xl font-black text-foreground"
        >
          <img src="/logo.svg" alt="" className="h-9 w-9 rounded-full object-cover" />
          <span>{loginT("brandName")}</span>
        </Link>
        <ThemeToggle />
      </header>

      <main className="flex flex-1 items-center justify-center p-4">
        <div className="w-full max-w-sm animate-fade-in space-y-6 rounded-2xl border border-border bg-card p-8">
          <Link
            href={`/${locale}`}
            className="inline-flex min-h-11 items-center gap-1 text-xs font-semibold text-muted-foreground hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          >
            <ArrowLeft aria-hidden="true" className="h-3.5 w-3.5" /> {loginT("backHome")}
          </Link>

          <div className="space-y-2">
            <h1 className="font-display text-2xl font-extrabold tracking-tight">{t("title")}</h1>
            <p className="text-sm text-muted-foreground">{t("subtitle")}</p>
          </div>

          <form onSubmit={handleSubmit} className="space-y-4">
            <div className="space-y-1.5">
              <label
                htmlFor="register-phone"
                className="text-[11px] font-extrabold uppercase tracking-wider text-muted-foreground"
              >
                {t("phoneLabel")}
              </label>
              <div className="flex items-center gap-2 rounded-2xl border border-border bg-background px-4 py-3 text-sm transition-colors focus-within:border-accent focus-within:ring-2 focus-within:ring-ring">
                <Phone aria-hidden="true" className="h-4 w-4 shrink-0 text-muted-foreground" />
                <span className="font-bold text-foreground">+998</span>
                <input
                  id="register-phone"
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
                htmlFor="register-name"
                className="text-[11px] font-extrabold uppercase tracking-wider text-muted-foreground"
              >
                {t("nameLabel")}
              </label>
              <div className="flex items-center gap-2 rounded-2xl border border-border bg-background px-4 py-3 text-sm transition-colors focus-within:border-accent focus-within:ring-2 focus-within:ring-ring">
                <User aria-hidden="true" className="h-4 w-4 shrink-0 text-muted-foreground" />
                <input
                  id="register-name"
                  type="text"
                  autoComplete="name"
                  value={name}
                  onChange={(e) => setName(e.target.value.slice(0, 80))}
                  placeholder={t("namePlaceholder")}
                  className="w-full bg-transparent font-bold tracking-wide outline-none placeholder:font-normal placeholder:text-muted-foreground"
                  aria-label={t("nameLabel")}
                />
              </div>
            </div>

            <div className="space-y-1.5">
              <label
                htmlFor="register-password"
                className="text-[11px] font-extrabold uppercase tracking-wider text-muted-foreground"
              >
                {t("passwordLabel")}
              </label>
              <div className="flex items-center gap-2 rounded-2xl border border-border bg-background px-4 py-3 text-sm transition-colors focus-within:border-accent focus-within:ring-2 focus-within:ring-ring">
                <Lock aria-hidden="true" className="h-4 w-4 shrink-0 text-muted-foreground" />
                <input
                  id="register-password"
                  type="password"
                  autoComplete="new-password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  placeholder={t("passwordPlaceholder")}
                  className="w-full bg-transparent font-bold tracking-wide outline-none placeholder:font-normal placeholder:text-muted-foreground"
                  aria-label={t("passwordLabel")}
                />
              </div>
            </div>

            <div className="space-y-1.5">
              <label
                htmlFor="register-confirm"
                className="text-[11px] font-extrabold uppercase tracking-wider text-muted-foreground"
              >
                {t("confirmPasswordLabel")}
              </label>
              <div className="flex items-center gap-2 rounded-2xl border border-border bg-background px-4 py-3 text-sm transition-colors focus-within:border-accent focus-within:ring-2 focus-within:ring-ring">
                <Lock aria-hidden="true" className="h-4 w-4 shrink-0 text-muted-foreground" />
                <input
                  id="register-confirm"
                  type="password"
                  autoComplete="new-password"
                  value={confirmPassword}
                  onChange={(e) => setConfirmPassword(e.target.value)}
                  placeholder={t("confirmPasswordPlaceholder")}
                  className="w-full bg-transparent font-bold tracking-wide outline-none placeholder:font-normal placeholder:text-muted-foreground"
                  aria-label={t("confirmPasswordLabel")}
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
              {submitting ? t("submitting") : t("submit")}
            </Button>
          </form>

          <p className="text-center text-xs text-muted-foreground">
            {t("haveAccount")}{" "}
            <Link
              href={`/${locale}/login`}
              className="font-bold text-foreground underline-offset-2 hover:underline"
            >
              {t("loginLink")}
            </Link>
          </p>
        </div>
      </main>
    </div>
  );
}
