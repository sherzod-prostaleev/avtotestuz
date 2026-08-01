"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useTranslations } from "next-intl";
import { type ColumnDef } from "@tanstack/react-table";
import { AdminPageHeader } from "@/components/admin/admin-page-header";
import { AdminErrorState } from "@/components/admin/admin-error-state";
import { AdminSkeleton } from "@/components/admin/admin-skeleton";
import { PermissionGate } from "@/components/admin/permission-gate";
import { Button } from "@/components/ui/button";
import { useAdminMeOptional } from "@/components/admin/admin-me-context";
import { hasPermission } from "@/lib/admin-permissions";
import {
  AdminDataTable,
  type AdminColumnMeta,
} from "@/components/admin/admin-data-table";

type PayoutRow = {
  id: string;
  profile_id: string;
  profile_phone: string;
  profile_name: string;
  amount_uzs: number;
  card_masked: string;
  card_network: string;
  status: string;
  admin_note: string;
  created_at: string;
};

function formatSom(n: number): string {
  return String(n).replace(/\B(?=(\d{3})+(?!\d))/g, " ");
}

export default function AdminReferralPayoutsPage() {
  const t = useTranslations("AdminReferral");
  const [status, setStatus] = useState("pending");
  const [items, setItems] = useState<PayoutRow[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [busyId, setBusyId] = useState<string | null>(null);
  const [revealedCards, setRevealedCards] = useState<Record<string, string>>({});
  const [revealingId, setRevealingId] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const qs = status ? `?status=${encodeURIComponent(status)}` : "";
      const res = await fetch(`/api/admin/referral/payouts${qs}`, { cache: "no-store" });
      const json = await res.json();
      if (!res.ok) {
        setError(t("errorLoad"));
        setItems([]);
        return;
      }
      setItems((json.data?.items ?? []) as PayoutRow[]);
      setRevealedCards({});
    } catch {
      setError(t("errorLoad"));
    } finally {
      setLoading(false);
    }
  }, [status, t]);

  useEffect(() => {
    void load();
  }, [load]);

  const act = useCallback(
    async (id: string, action: "paid" | "reject") => {
      setBusyId(id);
      try {
        const res = await fetch(`/api/admin/referral/payouts/${id}/${action}`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ note: "" }),
        });
        if (!res.ok) {
          setError(t("errorAction"));
          return;
        }
        await load();
      } catch {
        setError(t("errorAction"));
      } finally {
        setBusyId(null);
      }
    },
    [load, t],
  );

  const me = useAdminMeOptional();
  const canManage = hasPermission(me?.permissions, "referral.payouts.manage");

  const revealCard = useCallback(async (id: string) => {
    if (revealedCards[id]) {
      setRevealedCards((current) => {
        const next = { ...current };
        delete next[id];
        return next;
      });
      return;
    }
    setRevealingId(id);
    setError(null);
    try {
      const res = await fetch(`/api/admin/referral/payouts/${id}/card`, { cache: "no-store" });
      const json = await res.json().catch(() => ({}));
      const full = json?.data?.card_number as string | undefined;
      if (!res.ok || !full) {
        setError(t("errorReveal"));
        return;
      }
      setRevealedCards((current) => ({ ...current, [id]: full }));
    } catch {
      setError(t("errorReveal"));
    } finally {
      setRevealingId(null);
    }
  }, [revealedCards, t]);

  const columns = useMemo<ColumnDef<PayoutRow>[]>(
    () => [
      {
        accessorKey: "profile_name",
        header: t("colUser"),
        meta: { cardTitle: true } satisfies AdminColumnMeta,
        cell: ({ row }) => (
          <div>
            <div className="font-medium">{row.original.profile_name || "—"}</div>
            <div className="text-xs text-muted-foreground">{row.original.profile_phone}</div>
          </div>
        ),
      },
      {
        accessorKey: "amount_uzs",
        header: t("colAmount"),
        meta: { numeric: true } satisfies AdminColumnMeta,
        cell: ({ row }) => <span className="font-mono">{formatSom(row.original.amount_uzs)}</span>,
      },
      {
        accessorKey: "card_masked",
        header: t("colCard"),
        cell: ({ row }) => (
          <div>
            <div className="font-mono text-xs">
              {revealedCards[row.original.id] ?? row.original.card_masked}
            </div>
            <div className="text-[11px] uppercase text-muted-foreground">
              {row.original.card_network}
            </div>
            <Button
              type="button"
              size="sm"
              variant="ghost"
              className="mt-1"
              disabled={revealingId === row.original.id}
              onClick={() => void revealCard(row.original.id)}
            >
              {revealedCards[row.original.id]
                ? t("hideCard")
                : revealingId === row.original.id
                  ? t("revealingCard")
                  : t("revealCard")}
            </Button>
          </div>
        ),
      },
      {
        accessorKey: "status",
        header: t("colStatus"),
        cell: ({ row }) => row.original.status,
      },
      {
        accessorKey: "created_at",
        header: t("colCreated"),
        cell: ({ row }) => (
          <span className="text-xs text-muted-foreground">
            {new Date(row.original.created_at).toLocaleString()}
          </span>
        ),
      },
      // Spread, not a gate inside the cell: gating only the contents leaves the
      // column standing, so a read-only operator got an empty Actions column on
      // desktop and a labelled blank on every phone card.
      ...(canManage
        ? ([{
        id: "actions",
        header: t("colActions"),
        cell: ({ row }) =>
          row.original.status === "pending" ? (
            <div className="flex gap-2">
                <Button
                  type="button"
                  size="sm"
                  variant="success"
                  disabled={busyId === row.original.id}
                  onClick={() => void act(row.original.id, "paid")}
                >
                  {t("markPaid")}
                </Button>
                <Button
                  type="button"
                  size="sm"
                  variant="outline"
                  disabled={busyId === row.original.id}
                  onClick={() => void act(row.original.id, "reject")}
                >
                  {t("reject")}
                </Button>
            </div>
          ) : null,
      }] as ColumnDef<PayoutRow>[])
        : []),
    ],
    [t, busyId, act, canManage, revealedCards, revealingId, revealCard],
  );

  return (
    <PermissionGate permission="referral.payouts.manage">
      <main className="space-y-6">
      <AdminPageHeader title={t("title")} description={t("subtitle")} />

      <div className="flex flex-wrap gap-2">
        {(["pending", "paid", "rejected", ""] as const).map((s) => (
          <Button
            key={s || "all"}
            type="button"
            size="sm"
            variant={status === s ? "game" : "outline"}
            onClick={() => setStatus(s)}
          >
            {s === "" ? t("filterAll") : t(`filter_${s}` as "filter_pending")}
          </Button>
        ))}
      </div>

      {loading && <AdminSkeleton rows={6} />}
      {error && <AdminErrorState message={error} onRetry={() => void load()} />}

      {!loading && !error && (
        <AdminDataTable
          data={items}
          columns={columns}
          emptyTitle={t("empty")}
          getRowId={(row) => row.id}
        />
      )}
      </main>
    </PermissionGate>
  );
}
