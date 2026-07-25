"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { useLocale, useTranslations } from "next-intl";
import { useParams } from "next/navigation";
import { ArrowLeft } from "lucide-react";
import { Button } from "@/components/ui/button";

type Ticket = {
  id: string;
  profile_id?: string;
  contact_email: string;
  contact_phone: string;
  subject: string;
  body: string;
  status: string;
  locale: string;
  source: string;
  admin_note: string;
  created_at: string;
  updated_at: string;
};

const STATUSES = ["open", "in_progress", "resolved", "closed"] as const;

export default function AdminSupportInboxDetailPage() {
  const t = useTranslations("AdminInbox");
  const locale = useLocale();
  const params = useParams();
  const id = String(params.id ?? "");
  const [ticket, setTicket] = useState<Ticket | null>(null);
  const [status, setStatus] = useState("open");
  const [note, setNote] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const load = useCallback(async () => {
    setError(null);
    try {
      const res = await fetch(`/api/admin/support/tickets/${id}`, { cache: "no-store" });
      const json = await res.json();
      if (!res.ok) {
        setError(json?.error?.code === "forbidden" ? t("errorForbidden") : t("errorLoad"));
        setTicket(null);
        return;
      }
      const data = json.data as Ticket;
      setTicket(data);
      setStatus(data.status);
      setNote(data.admin_note ?? "");
    } catch {
      setError(t("errorLoad"));
      setTicket(null);
    }
  }, [id, t]);

  useEffect(() => {
    void load();
  }, [load]);

  async function save() {
    setBusy(true);
    setError(null);
    try {
      const res = await fetch(`/api/admin/support/tickets/${id}`, {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ status, admin_note: note }),
      });
      const json = await res.json();
      if (!res.ok) {
        setError(json?.error?.code === "forbidden" ? t("errorForbidden") : t("errorSave"));
        return;
      }
      const data = json.data as Ticket;
      setTicket(data);
      setStatus(data.status);
      setNote(data.admin_note ?? "");
    } catch {
      setError(t("errorSave"));
    } finally {
      setBusy(false);
    }
  }

  return (
    <main className="mx-auto max-w-2xl space-y-4">
      <Link
        href={`/${locale}/admin/support/inbox`}
        className="inline-flex items-center gap-1 text-sm font-semibold text-muted-foreground hover:text-accent"
      >
        <ArrowLeft aria-hidden className="h-4 w-4" />
        {t("back")}
      </Link>

      <header>
        <h1 className="font-display text-2xl font-extrabold tracking-tight">{t("detailTitle")}</h1>
        <p className="mt-1 text-sm text-muted-foreground">{t("note")}</p>
      </header>

      {error ? <p className="text-sm text-destructive">{error}</p> : null}

      {!ticket && !error ? (
        <p className="text-sm text-muted-foreground">{t("loading")}</p>
      ) : null}

      {ticket ? (
        <section className="space-y-4 rounded-xl border border-border bg-card p-4">
          <div>
            <p className="text-xs font-bold uppercase tracking-wide text-muted-foreground">
              {t("colSubject")}
            </p>
            <p className="mt-1 font-semibold">{ticket.subject}</p>
          </div>
          <div>
            <p className="text-xs font-bold uppercase tracking-wide text-muted-foreground">
              {t("body")}
            </p>
            <p className="mt-1 whitespace-pre-wrap text-sm">{ticket.body}</p>
          </div>
          <dl className="grid gap-2 text-sm sm:grid-cols-2">
            <div>
              <dt className="text-xs text-muted-foreground">{t("contact")}</dt>
              <dd>
                {ticket.contact_email || "—"}
                {ticket.contact_phone ? ` · ${ticket.contact_phone}` : ""}
              </dd>
            </div>
            <div>
              <dt className="text-xs text-muted-foreground">{t("meta")}</dt>
              <dd>
                {ticket.source} · {ticket.locale}
                {ticket.profile_id ? ` · ${ticket.profile_id.slice(0, 8)}…` : ""}
              </dd>
            </div>
            <div>
              <dt className="text-xs text-muted-foreground">{t("colCreated")}</dt>
              <dd>{new Date(ticket.created_at).toLocaleString()}</dd>
            </div>
            <div>
              <dt className="text-xs text-muted-foreground">{t("updated")}</dt>
              <dd>{new Date(ticket.updated_at).toLocaleString()}</dd>
            </div>
          </dl>

          <label className="block space-y-1">
            <span className="text-xs font-bold text-muted-foreground">{t("status")}</span>
            <select
              value={status}
              onChange={(e) => setStatus(e.target.value)}
              className="h-10 w-full rounded-xl border border-border bg-background px-3 text-sm"
            >
              {STATUSES.map((s) => (
                <option key={s} value={s}>
                  {t(`status_${s}` as "status_open")}
                </option>
              ))}
            </select>
          </label>

          <label className="block space-y-1">
            <span className="text-xs font-bold text-muted-foreground">{t("adminNote")}</span>
            <textarea
              value={note}
              onChange={(e) => setNote(e.target.value)}
              rows={3}
              className="w-full rounded-xl border border-border bg-background px-3 py-2 text-sm"
            />
          </label>

          <Button type="button" size="sm" disabled={busy} onClick={() => void save()}>
            {t("save")}
          </Button>
        </section>
      ) : null}
    </main>
  );
}
