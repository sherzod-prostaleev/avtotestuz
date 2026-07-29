"use client";

import { useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import { useRouter } from "next/navigation";
import { Button } from "@/components/ui/button";

export default function AdminLoginPage() {
  const t = useTranslations("AdminLogin");
  const locale = useLocale();
  const router = useRouter();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [totpCode, setTotpCode] = useState("");
  const [challenge, setChallenge] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      if (challenge) {
        const res = await fetch("/api/admin/auth/totp/verify", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ challenge_token: challenge, code: totpCode }),
        });
        const json = await res.json();
        if (!res.ok) {
          setError(json?.error?.code === "invalid_totp" ? t("errorTotp") : t("errorGeneric"));
          return;
        }
        router.replace(`/${locale}/admin`);
        return;
      }

      const res = await fetch("/api/admin/auth/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email, password }),
      });
      const json = await res.json();
      if (!res.ok) {
        setError(json?.error?.code === "invalid_credentials" ? t("errorCreds") : t("errorGeneric"));
        return;
      }
      if (json?.data?.requires_totp && json?.data?.challenge_token) {
        setChallenge(json.data.challenge_token as string);
        return;
      }
      router.replace(`/${locale}/admin`);
    } catch {
      setError(t("errorGeneric"));
    } finally {
      setBusy(false);
    }
  }

  return (
    <main className="relative flex min-h-screen items-center justify-center overflow-hidden bg-background px-4 py-10">
      <div
        aria-hidden
        className="pointer-events-none absolute inset-0 opacity-[0.08]"
        style={{
          backgroundImage:
            "linear-gradient(90deg, transparent 48%, hsl(var(--accent)) 50%, transparent 52%), linear-gradient(hsl(var(--border)) 1px, transparent 1px)",
          backgroundSize: "56px 56px, 100% 28px",
        }}
      />
      <div className="relative w-full max-w-md rounded-3xl border border-border/70 bg-card/70 p-6 shadow-2xl backdrop-blur">
        <p className="font-display text-2xl font-black tracking-tight">Driver Go</p>
        <p className="mt-1 text-[11px] font-extrabold uppercase tracking-[0.18em] text-accent">
          {t("badge")}
        </p>
        <h1 className="mt-4 font-display text-xl font-extrabold">{t("title")}</h1>
        <p className="mt-2 text-sm text-muted-foreground">{t("subtitle")}</p>

        <form className="mt-8 space-y-4" onSubmit={(e) => void onSubmit(e)}>
          {!challenge ? (
            <>
              <label className="block space-y-1.5">
                <span className="text-xs font-semibold text-muted-foreground">{t("email")}</span>
                <input
                  type="text"
                  inputMode="text"
                  autoComplete="username"
                  required
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  className="h-11 w-full rounded-xl border border-border bg-background px-3 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                />
              </label>
              <label className="block space-y-1.5">
                <span className="text-xs font-semibold text-muted-foreground">{t("password")}</span>
                <input
                  type="password"
                  autoComplete="current-password"
                  required
                  minLength={8}
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  className="h-11 w-full rounded-xl border border-border bg-background px-3 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                />
              </label>
            </>
          ) : (
            <label className="block space-y-1.5">
              <span className="text-xs font-semibold text-muted-foreground">{t("totpCode")}</span>
              <input
                inputMode="numeric"
                autoComplete="one-time-code"
                required
                minLength={6}
                maxLength={8}
                value={totpCode}
                onChange={(e) => setTotpCode(e.target.value)}
                className="h-11 w-full rounded-xl border border-border bg-background px-3 font-mono text-sm tracking-widest focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                placeholder="000000"
              />
              <p className="text-[11px] text-muted-foreground">{t("totpHint")}</p>
            </label>
          )}
          {error ? (
            <p
              role="alert"
              className="rounded-xl border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive"
            >
              {error}
            </p>
          ) : null}
          <Button type="submit" className="w-full" disabled={busy}>
            {busy ? t("submitting") : challenge ? t("totpSubmit") : t("submit")}
          </Button>
          {challenge ? (
            <Button
              type="button"
              variant="ghost"
              className="w-full"
              disabled={busy}
              onClick={() => {
                setChallenge(null);
                setTotpCode("");
              }}
            >
              {t("backToPassword")}
            </Button>
          ) : null}
        </form>
      </div>
    </main>
  );
}
