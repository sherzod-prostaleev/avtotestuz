"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useLocale, useTranslations } from "next-intl";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { ThemeToggle } from "@/components/theme-toggle";
import { ArrowLeft, CarFront, Phone, ShieldCheck } from "lucide-react";

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
    <div className="flex min-h-screen flex-col bg-background">
      {/* Mini top bar */}
      <header className="flex h-14 items-center justify-between px-4 border-b border-border/40">
        <Link href={`/${locale}`} className="flex items-center gap-2 font-display text-lg font-bold">
          <span className="flex h-8 w-8 items-center justify-center rounded-xl bg-accent text-white shadow-3d">
            <CarFront aria-hidden="true" className="h-4 w-4" />
          </span>
          <span className="bg-gradient-to-r from-accent to-indigo-500 bg-clip-text text-transparent">
            {t("brandName")}
          </span>
        </Link>
        <ThemeToggle />
      </header>

      {/* Center card */}
      <main className="flex flex-1 items-center justify-center p-4">
        <Card className="glass-card w-full max-w-sm p-8 space-y-6 shadow-2xl border-accent/20 animate-fade-in">
          <Link href={`/${locale}`} className="inline-flex items-center gap-1 text-xs font-semibold text-accent hover:underline">
            <ArrowLeft aria-hidden="true" className="h-3.5 w-3.5" /> {t("backHome")}
          </Link>

          <div className="text-center space-y-2">
            <div className="mx-auto flex h-14 w-14 items-center justify-center rounded-2xl bg-accent text-white font-display text-2xl font-black shadow-3d">
              <CarFront aria-hidden="true" className="h-7 w-7" />
            </div>
            <h1 className="font-display text-2xl font-extrabold tracking-tight">{t("title")}</h1>
            <p className="text-xs text-muted-foreground">{t("subtitle")}</p>
          </div>

          <form onSubmit={handleSubmit} className="space-y-4">
            <div className="space-y-1.5">
              <label htmlFor="login-phone" className="text-[11px] font-extrabold text-muted-foreground uppercase tracking-wider">
                {t("phoneLabel")}
              </label>
              <div className="flex items-center gap-2 rounded-2xl border border-border bg-card px-4 py-3 text-sm focus-within:border-accent focus-within:ring-2 focus-within:ring-accent/20 transition-all">
                <Phone aria-hidden="true" className="h-4 w-4 text-muted-foreground shrink-0" />
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
              <div role="alert" className="rounded-xl border border-danger/50 bg-danger/10 p-3 text-xs font-semibold text-danger">
                {t(ERROR_MESSAGE_KEYS[error] ?? "errorUnknown")}
              </div>
            )}

            <Button type="submit" variant="game" size="lg" className="w-full py-3 text-sm font-extrabold" disabled={phone.length !== 9 || submitting}>
              {submitting ? t("submitting") : t("continue")}
            </Button>
          </form>

          <div className="flex items-center justify-center gap-1.5 text-[11px] text-muted-foreground">
            <ShieldCheck aria-hidden="true" className="h-3.5 w-3.5 text-success" />
            <span>{t("secureSms")}</span>
          </div>
        </Card>
      </main>
    </div>
  );
}
