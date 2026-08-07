"use client";

import { useCallback, useEffect, useState } from "react";
import Image from "next/image";
import Link from "next/link";
import { useLocale, useTranslations } from "next-intl";
import { apiGet } from "@/lib/api-client";
import { Card, CardTitle, CardDescription } from "@/components/ui/card";
import {
  Award,
  BarChart3,
  Bookmark,
  BookOpen,
  ChevronRight,
  Signpost,
  Target,
  Trophy,
} from "lucide-react";

// GET /me answers {"data":{"profile":{...},"vip":{...}}} and apiGet unwraps
// only "data", so the profile is one level down -- the same shape
// (app)/profile/page.tsx and use-user-stats.ts already read. Reading kind off
// the top level yields undefined against the real backend, which is how every
// classroom PC was told it was not a classroom PC.
//
// `kind` is the only field that distinguishes a station: a shadow profile's
// `role` is the default "user" like any learner's.
type Me = { profile?: { kind?: string; name?: string } };

// The three entry points a lesson actually starts from, in the same order and
// with the same icons and accent colours the learner dashboard uses. A student
// who studies at home on their own account should recognise this screen, not
// have to relearn it -- which is the whole reason the kiosk reuses the learner
// pages rather than reimplementing them.
const PRIMARY = [
  {
    key: "tickets",
    href: "station/tickets",
    icon: BookOpen,
    titleKey: "Station.tickets",
    descKey: "Dashboard.navVariantsDesc",
    metaKey: "Dashboard.variantsMeta",
    tone: "text-accent",
    chip: "bg-accent/15 text-accent",
  },
  {
    key: "exam",
    href: "station/session/start?mode=exam",
    icon: Award,
    titleKey: "Station.exam",
    descKey: "Dashboard.navExamDesc",
    metaKey: "Dashboard.examMeta",
    tone: "text-accent",
    chip: "bg-accent/15 text-accent",
  },
  {
    key: "practice",
    href: "station/practice",
    icon: Target,
    titleKey: "Station.practice",
    descKey: "Dashboard.navPracticeDesc",
    metaKey: "Dashboard.practiceMeta",
    tone: "text-success",
    chip: "bg-success/15 text-success",
  },
] as const;

// The reference shelf: nothing here starts a timed session, so it sits below
// the three study modes rather than competing with them.
const SECONDARY = [
  {
    key: "stats",
    href: "station/stats",
    icon: BarChart3,
    titleKey: "Station.stats",
    descKey: "Stats.subtitle",
  },
  {
    key: "saved",
    href: "station/saved",
    icon: Bookmark,
    titleKey: "Station.saved",
    descKey: "Saved.subtitle",
  },
  {
    key: "leaderboard",
    href: "station/leaderboard",
    icon: Trophy,
    titleKey: "Station.leaderboard",
    // Not Leaderboard.subtitle, which invites the reader to compete for
    // points: a station is excluded from the rankings on purpose
    // (leaderboard.Service.RecordPoint returns early for a kind='station'
    // profile), so this screen has to say the board is read-only here rather
    // than promise a place on it.
    descKey: "Station.leaderboardNote",
  },
] as const;

export default function StationPage() {
  const t = useTranslations();
  const locale = useLocale();
  const [me, setMe] = useState<Me | null>(null);
  const [checked, setChecked] = useState(false);

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
    return <p className="p-8 text-center text-lg">{t("Station.notStation")}</p>;
  }

  const stationName = me.profile.name ?? "";

  return (
    // pt-16 clears the fixed language/theme controls in the top-right corner
    // (see (kiosk)/kiosk-chrome.tsx), which would otherwise sit on the title.
    <main className="mx-auto max-w-6xl space-y-6 px-4 pb-10 pt-16 sm:space-y-8 sm:px-6 sm:pt-20">
      <header className="flex items-center gap-3 sm:gap-4">
        <Image
          src="/logo.svg"
          alt=""
          width={56}
          height={56}
          className="h-11 w-11 shrink-0 rounded-2xl sm:h-14 sm:w-14"
          priority
        />
        <div className="min-w-0">
          <h1 className="font-display text-2xl font-extrabold tracking-tight sm:text-3xl">
            {t("Station.title")}
          </h1>
          {stationName ? (
            <p className="truncate text-sm text-muted-foreground sm:text-base">{stationName}</p>
          ) : null}
        </div>
      </header>

      <section className="space-y-2 sm:space-y-4">
        <h2 className="font-display text-lg font-bold tracking-tight sm:text-2xl">
          {t("Dashboard.modesTitle")}
        </h2>
        <div className="grid gap-3 sm:gap-4 md:grid-cols-3">
          {PRIMARY.map((item) => {
            const Icon = item.icon;
            return (
              <Link key={item.key} href={`/${locale}/${item.href}`} className="block min-w-0">
                <Card className="glass-card group flex h-full flex-col justify-between p-4 sm:p-6">
                  <div>
                    <div
                      className={`mb-3 flex h-11 w-11 items-center justify-center rounded-2xl transition-transform group-hover:scale-105 sm:h-12 sm:w-12 ${item.chip}`}
                    >
                      <Icon aria-hidden="true" className="h-5 w-5 sm:h-6 sm:w-6" />
                    </div>
                    <CardTitle className="text-base sm:text-xl">{t(item.titleKey)}</CardTitle>
                    <CardDescription className="mt-1.5 text-xs sm:text-sm">
                      {t(item.descKey)}
                    </CardDescription>
                  </div>
                  <div
                    className={`mt-4 flex items-center justify-between text-xs font-bold sm:text-sm ${item.tone}`}
                  >
                    <span className="truncate">{t(item.metaKey)}</span>
                    <ChevronRight
                      aria-hidden="true"
                      className="h-4 w-4 shrink-0 transition-transform group-hover:translate-x-1 sm:h-5 sm:w-5"
                    />
                  </div>
                </Card>
              </Link>
            );
          })}
        </div>
      </section>

      <section className="min-w-0">
        <Link href={`/${locale}/station/signs`}>
          <Card className="glass-card group border-border p-4 hover:border-accent sm:p-6">
            <div className="flex items-center justify-between gap-3">
              <div className="flex min-w-0 items-center gap-3 sm:gap-4">
                <div className="flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl bg-accent text-accent-foreground shadow-3d sm:h-14 sm:w-14">
                  <Signpost aria-hidden="true" className="h-6 w-6 sm:h-7 sm:w-7" />
                </div>
                <div className="min-w-0">
                  <h3 className="font-display text-base font-bold sm:text-xl">
                    {t("Station.signs")}
                  </h3>
                  <p className="mt-0.5 line-clamp-2 text-xs leading-snug text-muted-foreground sm:text-sm">
                    {t("Dashboard.signsDescription")}
                  </p>
                </div>
              </div>
              <ChevronRight
                aria-hidden="true"
                className="h-5 w-5 shrink-0 text-accent transition-transform group-hover:translate-x-1"
              />
            </div>
          </Card>
        </Link>
      </section>

      <section className="grid gap-3 sm:gap-4 md:grid-cols-3">
        {SECONDARY.map((item) => {
          const Icon = item.icon;
          return (
            <Link key={item.key} href={`/${locale}/${item.href}`} className="block min-w-0">
              <Card className="glass-card group flex h-full items-center gap-3 p-4 hover:border-accent sm:gap-4 sm:p-5">
                <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-muted text-muted-foreground transition-colors group-hover:text-accent sm:h-12 sm:w-12 sm:rounded-2xl">
                  <Icon aria-hidden="true" className="h-5 w-5 sm:h-6 sm:w-6" />
                </div>
                <div className="min-w-0">
                  <h3 className="font-display text-sm font-bold sm:text-base">{t(item.titleKey)}</h3>
                  <p className="mt-0.5 line-clamp-2 text-[11px] leading-snug text-muted-foreground sm:text-xs">
                    {t(item.descKey)}
                  </p>
                </div>
                <ChevronRight
                  aria-hidden="true"
                  className="ml-auto h-4 w-4 shrink-0 text-muted-foreground transition-transform group-hover:translate-x-1 group-hover:text-accent"
                />
              </Card>
            </Link>
          );
        })}
      </section>
    </main>
  );
}
