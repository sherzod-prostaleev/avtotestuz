"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { useLocale, useTranslations } from "next-intl";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import { ThemeToggle } from "@/components/theme-toggle";
import { ArrowLeft, Phone, ShieldCheck } from "lucide-react";
import { capturePendingReferralCodeFromUrl } from "@/lib/referral-storage";

const ERROR_MESSAGE_KEYS: Record<string, string> = {
  invalid_phone: "errorInvalidPhone",
  rate_limited: "errorRateLimited",
  network_error: "errorNetwork",
};

function normalizePhone(input: string): string {
  return input.replace(/\D/g, "");
}

export default function LoginPage() {
  const t = useTranslations("Login");
  const locale = useLocale();
  const router = useRouter();
  const [phone, setPhone] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  // Stash a `?ref=CODE` referral code from a shared invite link so it survives
  // the OTP round trip; verify/page.tsx redeems it once the account exists.
  useEffect(() => {
    capturePendingReferralCodeFromUrl();
  }, []);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      const res = await fetch("/api/auth/otp/request", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ phone: normalizePhone(phone), channel: "sandbox" }),
      });
      const json = await res.json();
      if (!res.ok) {
        setError(json.error?.code ?? "unknown");
        return;
      }
      const debugCode = json.data?.debug_code;
      const codeQuery = debugCode ? `&code=${encodeURIComponent(debugCode)}` : "";
      router.push(`/${locale}/login/verify?phone=${encodeURIComponent(normalizePhone(phone))}${codeQuery}`);
    } catch {
      setError("network_error");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="asphalt-hero flex min-h-screen flex-col bg-background">
      <header className="flex h-14 items-center justify-between border-b border-border px-4">
        <Link
          href={`/${locale}`}
          className="flex items-center gap-2.5 font-display text-xl font-black text-foreground"
        >
          <img src="/logo.svg" alt="" className="h-9 w-9 rounded-full object-cover" />
          <span>{t("brandName")}</span>
        </Link>
        <ThemeToggle />
      </header>

      <main className="flex flex-1 items-center justify-center p-4">
        <div className="w-full max-w-sm animate-fade-in space-y-6 rounded-2xl border border-border bg-card p-8">
          <Link
            href={`/${locale}`}
            className="inline-flex items-center gap-1 text-xs font-semibold text-muted-foreground hover:text-foreground"
          >
            <ArrowLeft aria-hidden="true" className="h-3.5 w-3.5" /> {t("backHome")}
          </Link>

          <div className="space-y-2">
            <h1 className="font-display text-2xl font-extrabold tracking-tight">{t("title")}</h1>
            <p className="text-sm text-muted-foreground">{t("subtitle")}</p>
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
              disabled={phone.length !== 9 || submitting}
            >
              {submitting ? t("submitting") : t("continue")}
            </Button>
          </form>

          <div className="flex items-center justify-center gap-1.5 text-[11px] text-muted-foreground">
            <ShieldCheck aria-hidden="true" className="h-3.5 w-3.5 text-success" />
            <span>{t("secureSms")}</span>
          </div>
        </div>
      </main>
    </div>
  );
}
