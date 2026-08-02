"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { useLocale, useTranslations } from "next-intl";
import { RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";

type OrgRow = {
  id: string;
  name: string;
  status: string;
  members?: number;
  active_seats?: number;
  created_at: string;
};

export default function AdminB2BOrgsPage() {
  const t = useTranslations("AdminB2B");
  const locale = useLocale();
  const [rows, setRows] = useState<OrgRow[] | null>(null);
  const [name, setName] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [creating, setCreating] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await fetch("/api/admin/b2b/orgs", { cache: "no-store" });
      const json = await res.json();
      if (!res.ok) {
        setError(json?.error?.code === "forbidden" ? t("errorForbiddenRead") : t("errorLoad"));
        setRows(null);
        return;
      }
      setRows(json.data as OrgRow[]);
    } catch {
      setError(t("errorLoad"));
      setRows(null);
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    void load();
  }, [load]);

  async function create() {
    setCreating(true);
    setError(null);
    try {
      const res = await fetch("/api/admin/b2b/orgs", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name }),
      });
      const json = await res.json();
      if (!res.ok) {
        setError(json?.error?.code === "forbidden" ? t("errorForbiddenGrant") : t("errorCreate"));
        return;
      }
      setName("");
      await load();
    } catch {
      setError(t("errorCreate"));
    } finally {
      setCreating(false);
    }
  }

  return (
    <main className="mx-auto max-w-2xl space-y-4">
      <header>
        <h1 className="font-display text-2xl font-extrabold tracking-tight">{t("title")}</h1>
        <p className="mt-1 text-sm text-muted-foreground">{t("subtitle")}</p>
        <p className="mt-2 text-xs text-muted-foreground">{t("note")}</p>
      </header>

      <div className="flex flex-wrap gap-2">
        <input
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder={t("namePlaceholder")}
          className="h-10 flex-1 rounded-xl border border-border bg-background px-3 text-sm"
        />
        <Button type="button" size="sm" disabled={creating || !name.trim()} onClick={() => void create()}>
          {t("create")}
        </Button>
        <Button type="button" size="sm" variant="outline" disabled={loading} onClick={() => void load()}>
          <RefreshCw aria-hidden className={`h-3.5 w-3.5 ${loading ? "animate-spin" : ""}`} />
        </Button>
      </div>

      {error ? <p className="text-sm text-destructive">{error}</p> : null}

      {!rows ? (
        <p className="text-sm text-muted-foreground">{t("loading")}</p>
      ) : rows.length === 0 ? (
        <p className="text-sm text-muted-foreground">{t("empty")}</p>
      ) : (
        <ul className="space-y-2">
          {rows.map((row) => (
            <li key={row.id}>
              <Link
                href={`/${locale}/admin/b2b/orgs/${row.id}`}
                className="flex items-center justify-between rounded-xl border border-border bg-card px-4 py-3 hover:border-accent"
              >
                <div>
                  <p className="font-bold">{row.name}</p>
                  <p className="text-xs text-muted-foreground">
                    {row.status === "active" ? t("statusActive") : t("statusSuspended")} ·{" "}
                    {t("membersSeats", { members: row.members ?? 0, seats: row.active_seats ?? 0 })}
                  </p>
                </div>
                <span className="text-xs font-semibold text-accent-ink">{t("open")}</span>
              </Link>
            </li>
          ))}
        </ul>
      )}
    </main>
  );
}
