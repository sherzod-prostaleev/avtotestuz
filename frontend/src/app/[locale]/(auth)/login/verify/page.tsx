"use client";

import { Suspense, useEffect, useRef, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";

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
  const router = useRouter();
  const searchParams = useSearchParams();
  const phone = searchParams.get("phone") ?? "";

  const [digits, setDigits] = useState<string[]>(Array(CODE_LENGTH).fill(""));
  const [error, setError] = useState<string | null>(null);
  const [cooldown, setCooldown] = useState(RESEND_COOLDOWN_SECONDS);
  const inputRefs = useRef<(HTMLInputElement | null)[]>([]);

  useEffect(() => {
    if (cooldown <= 0) return;
    const timer = setInterval(() => setCooldown((c) => Math.max(0, c - 1)), 1000);
    return () => clearInterval(timer);
  }, [cooldown]);

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
      router.push("/dashboard");
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
    await fetch("/api/auth/otp/request", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ phone, channel: "sandbox" }),
    });
  }

  return (
    <main className="flex min-h-screen items-center justify-center px-4">
      <Card className="w-full max-w-sm p-8 text-center">
        <h1 className="font-display text-2xl font-bold">{t("title")}</h1>
        <p className="mt-2 text-sm text-muted-foreground">{t("subtitle", { phone })}</p>
        <div className="mt-6 flex justify-center gap-2">
          {digits.map((d, i) => (
            <input
              key={i}
              ref={(el) => {
                inputRefs.current[i] = el;
              }}
              type="text"
              inputMode="numeric"
              value={d}
              onChange={(e) => handleDigitChange(i, e.target.value)}
              aria-label={t("digitLabel", { position: i + 1 })}
              className="h-12 w-10 rounded-md border border-border bg-card text-center text-lg font-bold outline-none focus:border-accent"
            />
          ))}
        </div>
        {error && <p className="mt-4 text-sm text-danger">{t(ERROR_MESSAGE_KEYS[error] ?? "errorUnknown")}</p>}
        <Button type="button" variant="outline" className="mt-6" disabled={cooldown > 0} onClick={handleResend}>
          {cooldown > 0 ? t("resendIn", { seconds: cooldown }) : t("resend")}
        </Button>
      </Card>
    </main>
  );
}

export default function VerifyPage() {
  return (
    <Suspense fallback={null}>
      <VerifyForm />
    </Suspense>
  );
}
