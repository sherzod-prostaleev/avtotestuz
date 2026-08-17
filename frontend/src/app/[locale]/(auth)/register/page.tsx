"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { useLocale, useTranslations } from "next-intl";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import { BrandLogo } from "@/components/brand/brand-logo";
import { ThemeToggle } from "@/components/theme-toggle";
import { LocaleSwitcher } from "@/components/locale-switcher";
import { ArrowLeft, Lock, Phone, User } from "lucide-react";
import { applyPendingReferralCode, capturePendingReferralCodeFromUrl } from "@/lib/referral-storage";
import { migrateDemoProgressOnLogin } from "@/lib/demo-progress-storage";
import {
  NATIONAL_PHONE_INPUT_MAX_LENGTH,
  formatNationalPhone,
  normalizeNationalPhone,
} from "@/lib/phone-format";

const ERROR_MESSAGE_KEYS: Record<string, string> = {
  invalid_phone: "errorInvalidPhone",
  phone_taken: "errorPhoneTaken",
  weak_password: "errorWeakPassword",
  password_mismatch: "errorPasswordMismatch",
  rate_limited: "errorRateLimited",
  network_error: "errorNetwork",
};

/** National 9-digit UZ mobile (strips optional 998 country code). */
function normalizePhone(input: string): string {
  return normalizeNationalPhone(input);
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
    const localPhone = normalizePhone(phone);
    if (localPhone.length !== 9) {
      setError("invalid_phone");
      return;
    }
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
      let res: Response;
      try {
        res = await fetch("/api/auth/register", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            phone: localPhone,
            password,
            name: name.trim() || undefined,
          }),
        });
      } catch {
        setError("network_error");
        return;
      }

      let responseCode = "unknown";
      try {
        const json = (await res.json()) as { error?: { code?: string } };
        responseCode = json.error?.code ?? "unknown";
      } catch {
        if (!res.ok) {
          setError("network_error");
          return;
        }
      }

      if (!res.ok) {
        setError(
          responseCode === "unknown" && res.status >= 500 ? "network_error" : responseCode,
        );
        return;
      }

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
        className="flex h-14 items-center justify-between border-b border-border px-3 sm:px-4"
        style={{ paddingTop: "env(safe-area-inset-top)" }}
      >
        <Link
          href={`/${locale}`}
          className="flex min-w-0 items-center gap-2 font-display text-lg font-black text-foreground sm:gap-2.5 sm:text-xl"
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
                  value={formatNationalPhone(phone)}
                  onChange={(e) => setPhone(normalizePhone(e.target.value))}
                  placeholder="90 123 45 67"
                  maxLength={NATIONAL_PHONE_INPUT_MAX_LENGTH}
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
              disabled={submitting}
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
