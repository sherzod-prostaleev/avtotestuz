"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useTranslations } from "next-intl";
import { RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { AdminPageHeader } from "@/components/admin/admin-page-header";
import { AdminErrorState } from "@/components/admin/admin-error-state";
import { AdminSkeleton } from "@/components/admin/admin-skeleton";
import { PermissionGate } from "@/components/admin/permission-gate";

type RBACPermission = {
  code: string;
  description: string;
};

type RBACRole = {
  code: string;
  name: string;
  description: string;
  permissions: string[];
};

type RBACMatrix = {
  roles: RBACRole[];
  permissions: RBACPermission[];
};

export default function AdminSecurityRBACPage() {
  const t = useTranslations("AdminRBAC");
  const tNav = useTranslations("AdminNav");
  const [data, setData] = useState<RBACMatrix | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await fetch("/api/admin/security/rbac", { cache: "no-store" });
      const json = await res.json();
      if (!res.ok) {
        setError(json?.error?.code === "forbidden" ? t("errorForbidden") : t("errorLoad"));
        setData(null);
        return;
      }
      setData(json.data as RBACMatrix);
    } catch {
      setError(t("errorLoad"));
      setData(null);
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    void load();
  }, [load]);

  const permSetByRole = useMemo(() => {
    const map = new Map<string, Set<string>>();
    for (const role of data?.roles ?? []) {
      map.set(role.code, new Set(role.permissions));
    }
    return map;
  }, [data]);

  return (
    <PermissionGate permission="security.rbac">
      <main className="mx-auto max-w-6xl space-y-5">
        <AdminPageHeader
          badge={tNav("groupSecurity")}
          title={t("title")}
          description={`${t("subtitle")} ${t("note")}`}
          actions={
            <Button type="button" size="sm" variant="outline" disabled={loading} onClick={() => void load()}>
              <RefreshCw aria-hidden className={`mr-1.5 h-3.5 w-3.5 ${loading ? "animate-spin" : ""}`} />
              {t("refresh")}
            </Button>
          }
        />

        {error ? (
          <AdminErrorState message={error} retryLabel={t("refresh")} onRetry={() => void load()} />
        ) : null}

        {!data && !error ? <AdminSkeleton rows={8} /> : null}

        {data ? (
          <>
            <section className="rounded-2xl border border-border/80 bg-card/70 p-4">
              <h2 className="text-sm font-bold">{t("rolesTitle")}</h2>
              <ul className="mt-3 grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
                {data.roles.map((role) => (
                  <li key={role.code} className="rounded-xl border border-border/70 bg-background/40 px-3 py-2.5">
                    <p className="font-mono text-sm font-bold">{role.code}</p>
                    <p className="text-xs font-semibold text-foreground/90">{role.name}</p>
                    {role.description ? (
                      <p className="mt-1 text-xs text-muted-foreground">{role.description}</p>
                    ) : null}
                    <p className="mt-1 text-[11px] text-muted-foreground">
                      {t("permCount", { count: role.permissions.length })}
                    </p>
                  </li>
                ))}
              </ul>
            </section>

            <section className="rounded-2xl border border-border/80 bg-card/70 p-4">
              <h2 className="text-sm font-bold">{t("matrixTitle")}</h2>
              <p className="mt-1 text-xs text-muted-foreground">{t("matrixHint")}</p>
              {/* A role × permission matrix is not a directory: there is no card
                  projection of it that stays readable, so it keeps its <table>
                  and instead scrolls inside its own focusable region. */}
              <div
                className="admin-scroll-x mt-3 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring"
                role="region"
                aria-label={t("matrixTitle")}
                tabIndex={0}
              >
                <table className="w-full min-w-max text-left text-xs">
                  <thead className="border-b border-border text-[10px] uppercase tracking-wide text-muted-foreground">
                    <tr>
                      <th className="sticky left-0 z-10 bg-card/95 px-2 py-2 font-bold">{t("colPermission")}</th>
                      {data.roles.map((role) => (
                        <th key={role.code} className="px-2 py-2 font-bold">
                          <span className="font-mono">{role.code}</span>
                        </th>
                      ))}
                    </tr>
                  </thead>
                  <tbody>
                    {data.permissions.map((perm) => (
                      <tr key={perm.code} className="border-b border-border/60 last:border-0">
                        <td className="sticky left-0 z-10 bg-card/95 px-2 py-2">
                          <p className="font-mono font-semibold">{perm.code}</p>
                          {perm.description ? (
                            <p className="mt-0.5 max-w-xs text-[10px] text-muted-foreground">{perm.description}</p>
                          ) : null}
                        </td>
                        {data.roles.map((role) => {
                          const granted = permSetByRole.get(role.code)?.has(perm.code);
                          return (
                            <td key={role.code} className="px-2 py-2 text-center">
                              <span
                                className={
                                  granted
                                    ? "inline-flex h-5 w-5 items-center justify-center rounded-md bg-emerald-500/20 text-emerald-300"
                                    : "inline-flex h-5 w-5 items-center justify-center rounded-md bg-muted/30 text-muted-foreground/40"
                                }
                                aria-label={granted ? t("granted") : t("denied")}
                              >
                                {granted ? "✓" : "·"}
                              </span>
                            </td>
                          );
                        })}
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </section>
          </>
        ) : null}
      </main>
    </PermissionGate>
  );
}
