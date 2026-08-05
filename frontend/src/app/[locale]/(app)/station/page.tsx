"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { useLocale, useTranslations } from "next-intl";
import { apiGet } from "@/lib/api-client";
import { Button } from "@/components/ui/button";

// `kind` is the only field that distinguishes a station: a shadow profile's
// `role` is the default "user" like any learner's.
type Me = { kind: string; name: string };

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

  if (!me || me.kind !== "station") {
    return <p className="p-8 text-center text-lg">{t("notStation")}</p>;
  }

  return (
    <main key={sitting} className="mx-auto max-w-2xl space-y-6 p-8">
      <h1 className="text-3xl font-bold">{t("title")}</h1>

      <div className="grid gap-4 sm:grid-cols-2">
        <Link href={`/${locale}/practice`} className="rounded-lg border p-6 text-center text-xl">
          {t("practice")}
        </Link>
        <Link href={`/${locale}/tickets`} className="rounded-lg border p-6 text-center text-xl">
          {t("exam")}
        </Link>
      </div>

      <Button className="w-full py-6 text-xl" onClick={() => setSitting((n) => n + 1)}>
        {t("newStudent")}
      </Button>
    </main>
  );
}
