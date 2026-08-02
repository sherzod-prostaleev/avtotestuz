"use client";

import { useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import { useRouter } from "next/navigation";
import { Button } from "@/components/ui/button";

/**
 * `password` -> `totp` is the ordinary sign-in. `enroll` is the recovery path
 * for an admin that ADMIN_TOTP_ENFORCE refuses a session to until they have an
 * authenticator: login answers 403 `totp_setup_required`, the BFF stashes the
 * scoped enrollment token in an httpOnly cookie, and the two calls below run
 * on that cookie alone. It lives here, not under /admin/(shell), because
 * proxy.ts bounces every /admin/* route without an admin session cookie —
 * which is exactly what an enrolling admin does not have yet.
 */
type Step = "password" | "totp" | "enroll";

export default function AdminLoginPage() {
  const t = useTranslations("AdminLogin");
  const locale = useLocale();
  const router = useRouter();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [totpCode, setTotpCode] = useState("");
  const [challenge, setChallenge] = useState<string | null>(null);
  const [step, setStep] = useState<Step>("password");
  const [enrollSecret, setEnrollSecret] = useState<string | null>(null);
  const [enrollOtpauth, setEnrollOtpauth] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  function backToPassword(message?: string) {
    setStep("password");
    setChallenge(null);
    setTotpCode("");
    setEnrollSecret(null);
    setEnrollOtpauth(null);
    setError(null);
    setNotice(message ?? null);
  }

  /** Fetches a fresh secret + otpauth URL; the secret is shown exactly once. */
  async function startEnrollment() {
    setBusy(true);
    setError(null);
    setEnrollSecret(null);
    setEnrollOtpauth(null);
    try {
      const res = await fetch("/api/admin/security/totp/enroll", { method: "POST" });
      const json = await res.json();
      if (!res.ok) {
        // 401 here means the 15-minute enrollment token is gone; sending the
        // admin back to the password step re-issues one instead of stranding
        // them on a step that can no longer succeed.
        setError(res.status === 401 ? t("enrollExpired") : t("errorEnroll"));
        return;
      }
      setEnrollSecret(json?.data?.secret as string);
      setEnrollOtpauth((json?.data?.otpauth_url as string) ?? null);
    } catch {
      setError(t("errorEnroll"));
    } finally {
      setBusy(false);
    }
  }

  async function confirmEnrollment() {
    const res = await fetch("/api/admin/security/totp/confirm", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      // Code only: the backend re-derives the secret it issued, so posting
      // one back would prove nothing.
      body: JSON.stringify({ code: totpCode }),
    });
    if (!res.ok) {
      setError(res.status === 401 ? t("enrollExpired") : t("errorEnrollCode"));
      if (res.status === 401) {
        setEnrollSecret(null);
        setEnrollOtpauth(null);
      }
      return;
    }
    backToPassword(t("enrollDone"));
  }

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    setNotice(null);
    try {
      if (step === "enroll") {
        await confirmEnrollment();
        return;
      }

      if (step === "totp") {
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
        if (json?.error?.code === "totp_setup_required") {
          setStep("enroll");
          setTotpCode("");
          setBusy(false);
          await startEnrollment();
          return;
        }
        setError(json?.error?.code === "invalid_credentials" ? t("errorCreds") : t("errorGeneric"));
        return;
      }
      if (json?.data?.requires_totp && json?.data?.challenge_token) {
        setChallenge(json.data.challenge_token as string);
        setStep("totp");
        return;
      }
      router.replace(`/${locale}/admin`);
    } catch {
      setError(t("errorGeneric"));
    } finally {
      setBusy(false);
    }
  }

  const submitLabel =
    step === "enroll" ? t("enrollSubmit") : step === "totp" ? t("totpSubmit") : t("submit");

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
        <p className="mt-1 text-[11px] font-extrabold uppercase tracking-[0.18em] text-accent-ink">
          {t("badge")}
        </p>
        <h1 className="mt-4 font-display text-xl font-extrabold">
          {step === "enroll" ? t("enrollTitle") : t("title")}
        </h1>
        <p className="mt-2 text-sm text-muted-foreground">
          {step === "enroll" ? t("enrollIntro") : t("subtitle")}
        </p>

        <form className="mt-8 space-y-4" onSubmit={(e) => void onSubmit(e)}>
          {step === "password" ? (
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
          ) : null}

          {step === "enroll" ? (
            enrollSecret ? (
              <div className="space-y-3">
                <p className="text-[11px] text-muted-foreground">{t("enrollSecretOnce")}</p>
                <code className="block break-all rounded-xl border border-border bg-background px-3 py-2 font-mono text-xs">
                  {enrollSecret}
                </code>
                {enrollOtpauth ? (
                  <p className="break-all font-mono text-[11px] text-muted-foreground">
                    <span className="sr-only">{t("enrollOtpauthLabel")}: </span>
                    {enrollOtpauth}
                  </p>
                ) : null}
              </div>
            ) : busy ? (
              <p className="text-sm text-muted-foreground">{t("enrollPreparing")}</p>
            ) : null
          ) : null}

          {(step === "totp" || (step === "enroll" && enrollSecret)) ? (
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
              <p className="text-[11px] text-muted-foreground">
                {step === "enroll" ? t("enrollCodeHint") : t("totpHint")}
              </p>
            </label>
          ) : null}

          {error ? (
            <p
              role="alert"
              className="rounded-xl border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive"
            >
              {error}
            </p>
          ) : null}
          {notice ? (
            <p
              role="status"
              className="rounded-xl border border-success/40 bg-success/10 px-3 py-2 text-sm text-success"
            >
              {notice}
            </p>
          ) : null}

          {step !== "enroll" || enrollSecret ? (
            <Button type="submit" className="w-full" disabled={busy}>
              {busy ? t("submitting") : submitLabel}
            </Button>
          ) : null}

          {step === "enroll" && !enrollSecret ? (
            <Button
              type="button"
              className="w-full"
              disabled={busy}
              onClick={() => void startEnrollment()}
            >
              {busy ? t("submitting") : t("enrollRetry")}
            </Button>
          ) : null}

          {step !== "password" ? (
            <Button
              type="button"
              variant="ghost"
              className="w-full"
              disabled={busy}
              onClick={() => backToPassword()}
            >
              {t("backToPassword")}
            </Button>
          ) : null}
        </form>
      </div>
    </main>
  );
}
