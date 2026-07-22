"use client";

import { useState } from "react";
import { useTranslations, useLocale } from "next-intl";
import Link from "next/link";
import { useSigns, SignItem } from "@/hooks/use-signs";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { ArrowLeft, Search, Signpost, X, Info } from "lucide-react";

export default function SignsPage() {
  const t = useTranslations("Signs");
  const locale = useLocale();

  const [activeGroup, setActiveGroup] = useState<string>("all");
  const [search, setSearch] = useState<string>("");
  const [activeModalSign, setActiveModalSign] = useState<SignItem | null>(null);

  const { signs, loading, error } = useSigns(activeGroup, search);

  const groups = [
    { code: "all", name: t("groupAll") },
    { code: "warning", name: t("groupWarning") },
    { code: "priority", name: t("groupPriority") },
    { code: "prohibitory", name: t("groupProhibitory") },
    { code: "mandatory", name: t("groupMandatory") },
    { code: "information", name: t("groupInformation") },
    { code: "service", name: t("groupService") },
    { code: "supplementary", name: t("groupSupplementary") },
  ];

  const defaultDescription =
    "Ushbu yo'l belgisining amal qilish tartibi va qoidalari O'zbekiston Respublikasi YHQ standartiga mos keladi.";

  return (
    <main className="mx-auto max-w-6xl px-4 py-8 space-y-6">
      <header className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
        <div>
          <Link href={`/${locale}/dashboard`} className="mb-2 inline-flex items-center gap-1 text-sm text-accent hover:underline">
            <ArrowLeft className="h-4 w-4" /> Bosh sahifaga qaytish
          </Link>
          <h1 className="font-display text-2xl font-bold">{t("title")}</h1>
          <p className="text-sm text-muted-foreground">{t("subtitle")}</p>
        </div>

        <div className="relative w-full sm:w-72">
          <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <input
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder={t("searchPlaceholder")}
            className="w-full rounded-2xl border border-border bg-card py-2.5 pl-9 pr-4 text-xs font-semibold outline-none focus:border-accent"
          />
        </div>
      </header>

      {/* Group Filter Tabs */}
      <div className="flex gap-2 overflow-x-auto pb-2 scrollbar-none">
        {groups.map((grp) => (
          <button
            key={grp.code}
            onClick={() => setActiveGroup(grp.code)}
            className={`whitespace-nowrap rounded-xl px-4 py-2 text-xs font-bold transition-all ${
              activeGroup === grp.code
                ? "bg-accent text-white shadow-3d"
                : "border border-border bg-card text-muted-foreground hover:text-foreground"
            }`}
          >
            {grp.name}
          </button>
        ))}
      </div>

      {error && (
        <div className="rounded-2xl border border-destructive/50 bg-destructive/10 p-4 text-sm text-destructive font-medium">
          {error}
        </div>
      )}

      {/* Signs Grid */}
      {loading ? (
        <div className="py-12 text-center text-sm text-muted-foreground animate-pulse">Yo'l belgilari yuklanmoqda...</div>
      ) : signs.length === 0 ? (
        <div className="py-12 text-center text-sm text-muted-foreground">{t("emptyState")}</div>
      ) : (
        <div className="grid gap-4 grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6">
          {signs.map((sign) => (
            <Card
              key={sign.id || sign.code}
              onClick={() => setActiveModalSign(sign)}
              className="glass-card flex cursor-pointer flex-col items-center justify-between p-4 text-center transition-all duration-200 hover:-translate-y-1 hover:shadow-lg hover:border-accent"
            >
              <div className="mb-2 flex h-20 w-20 items-center justify-center rounded-xl bg-black/5 p-2">
                {/* Dynamic media URLs are served by the backend and intentionally stay unoptimized. */}
                {/* eslint-disable-next-line @next/next/no-img-element */}
                <img src={sign.image_url} alt={sign.name} className="max-h-full max-w-full object-contain" />
              </div>
              <span className="rounded-full bg-accent/10 px-2 py-0.5 text-[10px] font-extrabold text-accent">
                {sign.code}
              </span>
              <p className="mt-2 text-xs font-bold text-foreground line-clamp-2">{sign.name}</p>
            </Card>
          ))}
        </div>
      )}

      {/* Sign Details Modal */}
      {activeModalSign && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4 backdrop-blur-sm">
          <Card className="relative w-full max-w-lg p-6 space-y-4">
            <button
              onClick={() => setActiveModalSign(null)}
              className="absolute right-4 top-4 rounded-full border border-border p-1 text-muted-foreground hover:bg-card hover:text-foreground"
            >
              <X className="h-5 w-5" />
            </button>

            <div className="flex items-center gap-3">
              <span className="rounded-full bg-accent/20 px-3 py-1 text-xs font-extrabold text-accent">
                {activeModalSign.code}
              </span>
              <h2 className="font-display text-xl font-bold">{activeModalSign.name}</h2>
            </div>

            <div className="flex justify-center p-4 bg-black/5 rounded-2xl">
              {/* Dynamic media URLs are served by the backend and intentionally stay unoptimized. */}
              {/* eslint-disable-next-line @next/next/no-img-element */}
              <img
                src={activeModalSign.image_url}
                alt={activeModalSign.name}
                className="max-h-48 object-contain"
              />
            </div>

            <div className="rounded-xl border border-border bg-background/50 p-4 text-xs leading-relaxed text-foreground">
              <h3 className="mb-1 font-bold text-accent uppercase tracking-wider">Tavsifi</h3>
              <p>{activeModalSign.description || defaultDescription}</p>
            </div>

            <div className="flex justify-end">
              <Button variant="outline" size="sm" onClick={() => setActiveModalSign(null)}>
                Yopish
              </Button>
            </div>
          </Card>
        </div>
      )}
    </main>
  );
}
