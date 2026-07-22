"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";

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
  const router = useRouter();
  const [phone, setPhone] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

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
      router.push(`/login/verify?phone=${encodeURIComponent(normalizePhone(phone))}`);
    } catch {
      setError("network_error");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <main className="flex min-h-screen items-center justify-center px-4">
      <Card className="w-full max-w-sm p-8">
        <h1 className="font-display text-2xl font-bold">AvtoTest</h1>
        <p className="mt-2 text-sm text-muted-foreground">{t("subtitle")}</p>
        <form onSubmit={handleSubmit} className="mt-6 flex flex-col gap-4">
          <div className="flex items-center gap-2 rounded-md border border-border px-3 py-2">
            <span className="text-muted-foreground">+998</span>
            <input
              type="tel"
              inputMode="numeric"
              value={phone}
              onChange={(e) => setPhone(normalizePhone(e.target.value).slice(0, 9))}
              placeholder="90 123 45 67"
              className="w-full bg-transparent outline-none"
              aria-label={t("phoneLabel")}
            />
          </div>
          {error && <p className="text-sm text-danger">{t(ERROR_MESSAGE_KEYS[error] ?? "errorUnknown")}</p>}
          <Button type="submit" variant="game" size="lg" disabled={phone.length !== 9 || submitting}>
            {t("continue")}
          </Button>
        </form>
      </Card>
    </main>
  );
}
