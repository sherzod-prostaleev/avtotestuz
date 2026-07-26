"use client";

import { useCallback, useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { AdminPageHeader } from "@/components/admin/admin-page-header";
import { AdminErrorState } from "@/components/admin/admin-error-state";
import { PermissionGate } from "@/components/admin/permission-gate";
import { Button } from "@/components/ui/button";

type CardRow = {
  id: string;
  pan_full: string;
  pan_last4: string;
  holder_name: string;
  sort_order: number;
  enabled: boolean;
};

type QueueRow = {
  payment_id: string;
  amount_uzs: number;
  pan_last4: string;
  holder_name: string;
  manual_state: string;
  profile_phone: string;
  assigned_at: string;
};

type EventRow = {
  id: string;
  raw_text: string;
  amount_uzs?: number;
  pan_last4?: string;
  created_at: string;
};

type TgSettings = {
  configured: boolean;
  has_api_id: boolean;
  has_api_hash: boolean;
  has_session: boolean;
  humo_bot_username: string;
  last_test_ok?: boolean;
  last_test_detail?: string;
};

function formatSom(n: number): string {
  return String(n).replace(/\B(?=(\d{3})+(?!\d))/g, " ");
}

export default function AdminManualPayPage() {
  const t = useTranslations("AdminManualPay");
  const tNav = useTranslations("AdminNav");
  const [error, setError] = useState<string | null>(null);
  const [cards, setCards] = useState<CardRow[]>([]);
  const [queue, setQueue] = useState<QueueRow[]>([]);
  const [events, setEvents] = useState<EventRow[]>([]);
  const [tg, setTg] = useState<TgSettings | null>(null);
  const [pan, setPan] = useState("");
  const [holder, setHolder] = useState("");
  const [apiId, setApiId] = useState("");
  const [apiHash, setApiHash] = useState("");
  const [session, setSession] = useState("");
  const [botUser, setBotUser] = useState("HUMOcardbot");
  const [busy, setBusy] = useState(false);

  const load = useCallback(async () => {
    setError(null);
    try {
      const [c, q, e, g] = await Promise.all([
        fetch("/api/admin/payments/manual/cards", { cache: "no-store" }),
        fetch("/api/admin/payments/manual/queue", { cache: "no-store" }),
        fetch("/api/admin/payments/manual/events", { cache: "no-store" }),
        fetch("/api/admin/payments/manual/telegram", { cache: "no-store" }),
      ]);
      if (![c, q, e, g].every((r) => r.ok)) {
        setError(t("errorLoad"));
        return;
      }
      setCards(((await c.json()).data ?? []) as CardRow[]);
      setQueue(((await q.json()).data ?? []) as QueueRow[]);
      setEvents(((await e.json()).data ?? []) as EventRow[]);
      const tgData = (await g.json()).data as TgSettings;
      setTg(tgData);
      setBotUser(tgData.humo_bot_username || "HUMOcardbot");
    } catch {
      setError(t("errorLoad"));
    }
  }, [t]);

  useEffect(() => {
    void load();
  }, [load]);

  async function addCard() {
    setBusy(true);
    try {
      const res = await fetch("/api/admin/payments/manual/cards", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ pan_full: pan, holder_name: holder, enabled: true }),
      });
      if (!res.ok) {
        setError(t("errorLoad"));
        return;
      }
      setPan("");
      setHolder("");
      await load();
    } finally {
      setBusy(false);
    }
  }

  async function removeCard(id: string) {
    setBusy(true);
    try {
      await fetch(`/api/admin/payments/manual/cards/${id}`, { method: "DELETE" });
      await load();
    } finally {
      setBusy(false);
    }
  }

  async function confirmPay(id: string) {
    setBusy(true);
    try {
      await fetch(`/api/admin/payments/manual/queue/${id}/confirm`, { method: "POST" });
      await load();
    } finally {
      setBusy(false);
    }
  }

  async function rejectPay(id: string) {
    setBusy(true);
    try {
      await fetch(`/api/admin/payments/manual/queue/${id}/reject`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ note: "" }),
      });
      await load();
    } finally {
      setBusy(false);
    }
  }

  async function ignoreEvent(id: string) {
    await fetch(`/api/admin/payments/manual/events/${id}/ignore`, { method: "POST" });
    await load();
  }

  async function saveTg() {
    setBusy(true);
    try {
      const res = await fetch("/api/admin/payments/manual/telegram", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          api_id: apiId,
          api_hash: apiHash,
          session,
          humo_bot_username: botUser,
        }),
      });
      if (!res.ok) {
        setError(t("errorLoad"));
        return;
      }
      setApiId("");
      setApiHash("");
      setSession("");
      await load();
    } finally {
      setBusy(false);
    }
  }

  async function testTg() {
    setBusy(true);
    try {
      await fetch("/api/admin/payments/manual/telegram/test", { method: "POST" });
      await load();
    } finally {
      setBusy(false);
    }
  }

  return (
    <PermissionGate permission="payments.read">
      <main className="mx-auto max-w-3xl space-y-8">
        <AdminPageHeader
          badge={tNav("groupPayments")}
          title={t("title")}
          description={t("subtitle")}
          actions={
            <Button type="button" size="sm" variant="outline" onClick={() => void load()}>
              {t("refresh")}
            </Button>
          }
        />
        {error ? <AdminErrorState message={error} onRetry={() => void load()} /> : null}

        <section className="space-y-3">
          <h2 className="text-sm font-semibold">{t("cardsTitle")}</h2>
          <div className="flex flex-col gap-2 sm:flex-row">
            <input
              className="flex-1 rounded-lg border border-border bg-background px-3 py-2 text-sm"
              placeholder={t("pan")}
              value={pan}
              onChange={(e) => setPan(e.target.value)}
            />
            <input
              className="flex-1 rounded-lg border border-border bg-background px-3 py-2 text-sm"
              placeholder={t("holder")}
              value={holder}
              onChange={(e) => setHolder(e.target.value)}
            />
            <Button type="button" disabled={busy} onClick={() => void addCard()}>
              {t("addCard")}
            </Button>
          </div>
          <ul className="space-y-2">
            {cards.map((c) => (
              <li
                key={c.id}
                className="flex items-center justify-between gap-3 rounded-xl border border-border/70 px-3 py-2 text-sm"
              >
                <div>
                  <p className="font-mono font-semibold">**** {c.pan_last4}</p>
                  <p className="text-xs text-muted-foreground">{c.holder_name}</p>
                </div>
                <Button type="button" size="sm" variant="outline" disabled={busy} onClick={() => void removeCard(c.id)}>
                  {t("delete")}
                </Button>
              </li>
            ))}
          </ul>
        </section>

        <section className="space-y-3">
          <h2 className="text-sm font-semibold">{t("queueTitle")}</h2>
          {queue.length === 0 ? (
            <p className="text-sm text-muted-foreground">{t("emptyQueue")}</p>
          ) : (
            <ul className="space-y-2">
              {queue.map((row) => (
                <li
                  key={row.payment_id}
                  className="flex flex-col gap-2 rounded-xl border border-border/70 px-3 py-3 text-sm sm:flex-row sm:items-center sm:justify-between"
                >
                  <div>
                    <p className="font-semibold">
                      {formatSom(row.amount_uzs)} · *{row.pan_last4} · {row.manual_state}
                    </p>
                    <p className="text-xs text-muted-foreground">{row.profile_phone}</p>
                  </div>
                  <div className="flex gap-2">
                    <Button type="button" size="sm" disabled={busy} onClick={() => void confirmPay(row.payment_id)}>
                      {t("confirm")}
                    </Button>
                    <Button
                      type="button"
                      size="sm"
                      variant="outline"
                      disabled={busy}
                      onClick={() => void rejectPay(row.payment_id)}
                    >
                      {t("reject")}
                    </Button>
                  </div>
                </li>
              ))}
            </ul>
          )}
        </section>

        <section className="space-y-3">
          <h2 className="text-sm font-semibold">{t("eventsTitle")}</h2>
          {events.length === 0 ? (
            <p className="text-sm text-muted-foreground">{t("emptyEvents")}</p>
          ) : (
            <ul className="space-y-2">
              {events.map((ev) => (
                <li key={ev.id} className="rounded-xl border border-border/70 px-3 py-2 text-xs">
                  <pre className="whitespace-pre-wrap font-sans text-muted-foreground">{ev.raw_text}</pre>
                  <Button type="button" size="sm" variant="outline" className="mt-2" onClick={() => void ignoreEvent(ev.id)}>
                    {t("ignore")}
                  </Button>
                </li>
              ))}
            </ul>
          )}
        </section>

        <section className="space-y-3">
          <h2 className="text-sm font-semibold">{t("tgTitle")}</h2>
          <p className="text-xs text-muted-foreground">
            {tg?.configured ? t("configured") : t("notConfigured")}
            {tg?.last_test_detail ? ` — ${tg.last_test_detail}` : ""}
          </p>
          <div className="grid gap-2 sm:grid-cols-2">
            <input
              className="rounded-lg border border-border bg-background px-3 py-2 text-sm"
              placeholder={t("apiId")}
              value={apiId}
              onChange={(e) => setApiId(e.target.value)}
            />
            <input
              className="rounded-lg border border-border bg-background px-3 py-2 text-sm"
              placeholder={t("apiHash")}
              value={apiHash}
              onChange={(e) => setApiHash(e.target.value)}
            />
            <input
              className="rounded-lg border border-border bg-background px-3 py-2 text-sm sm:col-span-2"
              placeholder={t("session")}
              value={session}
              onChange={(e) => setSession(e.target.value)}
            />
            <input
              className="rounded-lg border border-border bg-background px-3 py-2 text-sm sm:col-span-2"
              placeholder={t("botUser")}
              value={botUser}
              onChange={(e) => setBotUser(e.target.value)}
            />
          </div>
          <div className="flex gap-2">
            <Button type="button" disabled={busy} onClick={() => void saveTg()}>
              {t("save")}
            </Button>
            <Button type="button" variant="outline" disabled={busy} onClick={() => void testTg()}>
              {t("testTg")}
            </Button>
          </div>
        </section>
      </main>
    </PermissionGate>
  );
}
