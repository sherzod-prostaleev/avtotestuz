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
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
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
      router.replace(`/${locale}/admin`);
    } catch {
      setError(t("errorGeneric"));
    } finally {
      setBusy(false);
    }
  }

  return (
    <main className="mx-auto flex min-h-screen max-w-md flex-col justify-center px-4 py-10">
      <p className="font-display text-2xl font-black text-foreground">Driver Go</p>
      <h1 className="mt-2 font-display text-xl font-extrabold">{t("title")}</h1>
      <p className="mt-2 text-sm text-muted-foreground">{t("subtitle")}</p>

      <form className="mt-8 space-y-4" onSubmit={(e) => void onSubmit(e)}>
        <label className="block space-y-1.5">
          <span className="text-xs font-semibold text-muted-foreground">{t("email")}</span>
          <input
            type="email"
            autoComplete="username"
            required
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            className="h-11 w-full rounded-xl border border-border bg-card px-3 text-sm"
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
            className="h-11 w-full rounded-xl border border-border bg-card px-3 text-sm"
          />
        </label>
        {error ? (
          <p role="alert" className="rounded-xl border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
            {error}
          </p>
        ) : null}
        <Button type="submit" className="w-full" disabled={busy}>
          {busy ? t("submitting") : t("submit")}
        </Button>
      </form>
    </main>
  );
}
