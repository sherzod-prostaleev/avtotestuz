"use client";

import { useTranslations } from "next-intl";

export default function AdminStubOrOverviewPage({
  params,
}: {
  params: { stub?: string[] };
}) {
  const t = useTranslations("AdminOverview");
  const parts = params.stub ?? [];
  if (parts.length === 0) {
    const tiles = [
      { label: t("tileUsers"), hint: "M3-1 ✓" },
      { label: t("tileContent"), hint: "M3-2 ✓" },
      { label: t("tilePayments"), hint: "M3-3 ✓" },
      { label: t("tileCMS"), hint: "M3-4 ✓" },
      { label: t("tileMonitoring"), hint: "M3-5 ✓" },
      { label: t("tileAnalytics"), hint: "M3-6 ✓" },
      { label: t("tileSecurity"), hint: "M3-0+" },
    ];
    return (
      <main className="mx-auto max-w-4xl space-y-4">
        <h1 className="font-display text-2xl font-extrabold tracking-tight">{t("title")}</h1>
        <p className="text-sm leading-6 text-muted-foreground">{t("body")}</p>
        <div className="grid gap-3 sm:grid-cols-3">
          {tiles.map((tile) => (
            <div key={tile.label} className="rounded-xl border border-border bg-card px-4 py-3">
              <p className="text-sm font-bold">{tile.label}</p>
              <p className="text-xs text-muted-foreground">{tile.hint}</p>
            </div>
          ))}
        </div>
      </main>
    );
  }

  return (
    <main className="mx-auto max-w-2xl space-y-3">
      <h1 className="font-display text-xl font-extrabold">{t("stubTitle")}</h1>
      <p className="text-sm text-muted-foreground">
        {t("stubBody", { path: parts.join("/") })}
      </p>
    </main>
  );
}
