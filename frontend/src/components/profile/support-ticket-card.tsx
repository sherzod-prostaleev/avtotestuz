"use client";

import { useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import { MessageSquare } from "lucide-react";
import { apiPost } from "@/lib/api-client";
import { Card, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";

type Ticket = { id: string };

export function SupportTicketCard() {
  const t = useTranslations("SupportTicket");
  const locale = useLocale();
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
      await apiPost<Ticket>("me/support/tickets", {
        subject,
        body,
        locale,
      });
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
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-base">
          <MessageSquare className="h-4 w-4" aria-hidden />
          {t("title")}
        </CardTitle>
      </CardHeader>
      <form onSubmit={(e) => void submit(e)} className="space-y-3 px-6 pb-6">
        <p className="text-sm text-muted-foreground">{t("subtitle")}</p>
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
          rows={3}
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
        <Button type="submit" size="sm" disabled={busy || !subject.trim() || !body.trim()}>
          {busy ? t("sending") : t("submit")}
        </Button>
      </form>
    </Card>
  );
}
