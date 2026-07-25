"use client";

import { useState } from "react";
import Link from "next/link";
import { useLocale, useTranslations } from "next-intl";
import { LegalDocShell } from "@/components/legal/legal-doc-shell";
import { Button } from "@/components/ui/button";

type Ticket = { id: string };

export default function PublicSupportPage() {
  const t = useTranslations("PublicSupport");
  const tLanding = useTranslations("Landing");
  const locale = useLocale();
  const [email, setEmail] = useState("");
  const [phone, setPhone] = useState("");
  const [subject, setSubject] = useState("");
  const [body, setBody] = useState("");
  const [busy, setBusy] = useState(false);
  const [done, setDone] = useState(false);
  const [error, setError] = useState(false);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(false);
    setDone(false);
    try {
      const res = await fetch("/api/proxy/support/tickets", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          contact_email: email,
          contact_phone: phone,
          subject,
          body,
          locale,
        }),
      });
      const json = await res.json();
      if (!res.ok) {
        setError(true);
        return;
      }
      const _ = json.data as Ticket;
      setEmail("");
      setPhone("");
      setSubject("");
      setBody("");
      setDone(true);
    } catch {
      setError(true);
    } finally {
      setBusy(false);
    }
  }

  return (
    <LegalDocShell
      brandName={tLanding("brandName")}
      title={t("title")}
      updatedLabel={t("note")}
      backHref={`/${locale}`}
      backLabel={t("backHome")}
    >
      <p className="text-sm text-muted-foreground">{t("subtitle")}</p>
      <form onSubmit={(e) => void submit(e)} className="mt-4 space-y-3">
        <input
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          placeholder={t("email")}
          type="email"
          maxLength={120}
          className="field-input"
        />
        <input
          value={phone}
          onChange={(e) => setPhone(e.target.value)}
          placeholder={t("phone")}
          type="tel"
          maxLength={120}
          className="field-input"
        />
        <input
          value={subject}
          onChange={(e) => setSubject(e.target.value)}
          placeholder={t("subject")}
          maxLength={120}
          required
          className="field-input"
        />
        <textarea
          value={body}
          onChange={(e) => setBody(e.target.value)}
          placeholder={t("body")}
          maxLength={2000}
          required
          rows={4}
          className="w-full rounded-xl border border-border bg-background px-3 py-2 text-sm"
        />
        {done ? (
          <p role="status" className="text-sm font-medium text-success">
            {t("success")}
          </p>
        ) : null}
        {error ? (
          <p role="alert" className="text-sm text-destructive">
            {t("error")}
          </p>
        ) : null}
        <Button
          type="submit"
          size="sm"
          disabled={busy || !subject.trim() || !body.trim() || (!email.trim() && !phone.trim())}
        >
          {busy ? t("sending") : t("submit")}
        </Button>
      </form>
      <p className="mt-6 text-sm text-muted-foreground">
        <Link href={`/${locale}/login`} className="font-semibold text-accent hover:underline">
          {t("loginHint")}
        </Link>
      </p>
    </LegalDocShell>
  );
}
