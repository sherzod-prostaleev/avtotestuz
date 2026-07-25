"use client";

import { Suspense, useEffect, useRef, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { useLocale, useTranslations } from "next-intl";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import { ThemeToggle } from "@/components/theme-toggle";
import { KeyRound } from "lucide-react";
import { migrateDemoProgressOnLogin } from "@/lib/demo-progress-storage";
import { applyPendingReferralCode } from "@/lib/referral-storage";

const CODE_LENGTH = 6;
const RESEND_COOLDOWN_SECONDS = 60;

const ERROR_MESSAGE_KEYS: Record<string, string> = {
  invalid_code: "errorInvalidCode",
  expired_code: "errorExpiredCode",
  too_many_attempts: "errorTooManyAttempts",
  network_error: "errorNetwork",
};

function VerifyForm() {
  const t = useTranslations("Verify");
  const loginT = useTranslations("Login");
  const locale = useLocale();
  const router = useRouter();
  const searchParams = useSearchParams();
  const phone = searchParams.get("phone") ?? "";
  const initialCode = searchParams.get("code") ?? "";

  const [digits, setDigits] = useState<string[]>(() => {
    if (initialCode && initialCode.length === CODE_LENGTH) {
      return initialCode.split("");
    }
    return Array(CODE_LENGTH).fill("");
  });
  const [debugCode, setDebugCode] = useState<string>(initialCode);
  const [error, setError] = useState<string | null>(null);
  const [cooldown, setCooldown] = useState(RESEND_COOLDOWN_SECONDS);
  const inputRefs = useRef<(HTMLInputElement | null)[]>([]);

  useEffect(() => {
    if (cooldown <= 0) return;
    const timer = setInterval(() => setCooldown((c) => Math.max(0, c - 1)), 1000);
    return () => clearInterval(timer);
  }, [cooldown]);

  // Auto submit if initialCode is set
  useEffect(() => {
    if (initialCode && initialCode.length === CODE_LENGTH) {
      void submitCode(initialCode);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [initialCode]);

  async function submitCode(code: string) {
    setError(null);
    try {
      const res = await fetch("/api/auth/otp/verify", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ phone, code }),
      });
      const json = await res.json();
      if (!res.ok) {
        setError(json.error?.code ?? "unknown");
        return;
      }
      // Awaited, not fired and forgotten: navigating away mid-request would
      // abort it. A failure here is not fatal — ReferralCapture / DemoProgressCapture
      // retry anything still in storage on the next authenticated page load.
      await applyPendingReferralCode();
      await migrateDemoProgressOnLogin();
      router.push(`/${locale}/dashboard`);
    } catch {
      setError("network_error");
    }
  }

  function handleDigitChange(index: number, value: string) {
    const clean = value.replace(/\D/g, "").slice(-1);
    const next = [...digits];
    next[index] = clean;
    setDigits(next);

    if (clean && index < CODE_LENGTH - 1) {
      inputRefs.current[index + 1]?.focus();
    }
    if (next.every((d) => d !== "")) {
      void submitCode(next.join(""));
    }
  }

  async function handleResend() {
    setCooldown(RESEND_COOLDOWN_SECONDS);
    try {
      const res = await fetch("/api/auth/otp/request", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ phone, channel: "sandbox" }),
      });
      const json = await res.json();
      if (!res.ok) {
        setError(json.error?.code ?? "unknown");
        return;
      }
      if (json.data?.debug_code) {
        setDebugCode(json.data.debug_code);
        setDigits(json.data.debug_code.split(""));
        void submitCode(json.data.debug_code);
      }
    } catch {
      setError("network_error");
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
          <span>{loginT("brandName")}</span>
        </Link>
        <ThemeToggle />
      </header>

      <main className="flex flex-1 items-center justify-center px-4 py-8">
        <div className="w-full max-w-sm animate-fade-in space-y-6 rounded-2xl border border-border bg-card p-8 text-center">
          <div className="space-y-2">
            <h1 className="font-display text-2xl font-extrabold tracking-tight">{t("title")}</h1>
            <p className="text-sm text-muted-foreground">{t("subtitle", { phone })}</p>
          </div>

          {debugCode && (
            <div className="flex items-center justify-center gap-2 rounded-xl border border-border bg-background p-2.5 text-sm font-bold text-foreground">
              <KeyRound aria-hidden="true" className="h-4 w-4 text-accent" />
              <span>{t("debugCode", { code: debugCode })}</span>
            </div>
          )}

          <div role="group" aria-label={t("codeInputLabel")} className="flex justify-center gap-2">
            {digits.map((d, i) => (
              <input
                key={i}
                ref={(el) => {
                  inputRefs.current[i] = el;
                }}
                type="text"
                inputMode="numeric"
                autoComplete={i === 0 ? "one-time-code" : "off"}
                value={d}
                onChange={(e) => handleDigitChange(i, e.target.value)}
                aria-label={t("digitLabel", { position: i + 1 })}
                className="h-12 w-10 rounded-xl border border-border bg-background text-center text-lg font-bold outline-none focus:border-accent focus:ring-2 focus:ring-ring"
              />
            ))}
          </div>

          {error && (
            <p role="alert" className="text-sm font-semibold text-danger">
              {t(ERROR_MESSAGE_KEYS[error] ?? "errorUnknown")}
            </p>
          )}

          <Button type="button" variant="outline" className="w-full" disabled={cooldown > 0} onClick={handleResend}>
            {cooldown > 0 ? t("resendIn", { seconds: cooldown }) : t("resend")}
          </Button>
        </div>
      </main>
    </div>
  );
}

export default function VerifyPage() {
  const t = useTranslations("Verify");

  return (
    <Suspense
      fallback={
        <p role="status" className="p-6 text-center text-sm text-muted-foreground">
          {t("loading")}
        </p>
      }
    >
      <VerifyForm />
    </Suspense>
  );
}
