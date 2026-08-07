"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { useLocale, useTranslations } from "next-intl";
import { apiGet } from "@/lib/api-client";
import { Button } from "@/components/ui/button";

// GET /me answers {"data":{"profile":{...},"vip":{...}}} and apiGet unwraps
// only "data", so the profile is one level down -- the same shape
// (app)/profile/page.tsx and use-user-stats.ts already read. Reading kind off
// the top level yields undefined against the real backend, which is how every
// classroom PC was told it was not a classroom PC.
//
// `kind` is the only field that distinguishes a station: a shadow profile's
// `role` is the default "user" like any learner's.
type Me = { profile?: { kind?: string } };

export default function StationPage() {
  const t = useTranslations("Station");
  const locale = useLocale();
  const [me, setMe] = useState<Me | null>(null);
  const [checked, setChecked] = useState(false);
  // Bumping this key remounts the entry screen, which is what "next student"
  // means here: no server state to reset, just a clean screen.
  const [sitting, setSitting] = useState(0);

  const load = useCallback(async () => {
    try {
      setMe(await apiGet<Me>("me"));
    } catch {
      setMe(null);
    } finally {
      setChecked(true);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  if (!checked) return null;

  if (me?.profile?.kind !== "station") {
    return <p className="p-8 text-center text-lg">{t("notStation")}</p>;
  }

  return (
    <main key={sitting} className="mx-auto max-w-3xl space-y-6 p-8">
      <h1 className="text-3xl font-bold">{t("title")}</h1>

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        <Link href={`/${locale}/station/practice`} className="rounded-lg border p-6 text-center text-xl">
          {t("practice")}
        </Link>
        <Link href={`/${locale}/station/tickets`} className="rounded-lg border p-6 text-center text-xl">
          {t("tickets")}
        </Link>
        <Link href={`/${locale}/station/session/start?mode=exam`} className="rounded-lg border p-6 text-center text-xl">
          {t("exam")}
        </Link>
        <Link href={`/${locale}/station/signs`} className="rounded-lg border p-6 text-center text-xl">
          {t("signs")}
        </Link>
        <Link href={`/${locale}/station/stats`} className="rounded-lg border p-6 text-center text-xl">
          {t("stats")}
        </Link>
        <Link href={`/${locale}/station/saved`} className="rounded-lg border p-6 text-center text-xl">
          {t("saved")}
        </Link>
        <Link href={`/${locale}/station/leaderboard`} className="rounded-lg border p-6 text-center text-xl">
          {t("leaderboard")}
        </Link>
      </div>

      <Button className="w-full py-6 text-xl" onClick={() => setSitting((n) => n + 1)}>
        {t("newStudent")}
      </Button>
    </main>
  );
}
