"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useLocale, useTranslations } from "next-intl";
import { BrandLogo } from "@/components/brand/brand-logo";
import { apiGet } from "@/lib/api-client";
import { Card, CardTitle, CardDescription } from "@/components/ui/card";
import {
  Award,
  BarChart3,
  Bookmark,
  BookOpen,
  ChevronRight,
  LoaderCircle,
  Signpost,
  Target,
  TriangleAlert,
  Trophy,
} from "lucide-react";
import { StationDiagnostics, useStationStatus } from "./station-status";

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
    href: "station/exam",
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

// Phases of the station check.
//
// "waiting" exists because a classroom PC boots faster than its network. The
// agent autostarts, cannot reach the backend yet, and its proxy fails closed
// with 503 station_offline; it then keeps retrying in the background and
// succeeds a moment later. This page used to ask once, catch that 503, and
// show the "classroom computers only" refusal for the rest of the day even
// though the PC was a perfectly good station -- someone had to notice and
// press F5. A refusal is only correct when the backend actually answered and
// said this is not a station.
type Phase = "checking" | "waiting" | "station" | "refused";

// Backoff for the retry, in milliseconds. Fast at first because the usual wait
// is a few seconds of network coming up, then slower so a PC left on a dead
// network is not hammering its own agent all day.
const RETRY_MS = [1000, 2000, 3000, 5000, 8000, 10000];

export default function StationPage() {
  const t = useTranslations();
  const locale = useLocale();
  const [me, setMe] = useState<Me | null>(null);
  const [phase, setPhase] = useState<Phase>("checking");
  // Only polled while something is wrong: on a healthy PC this would be a
  // request every three seconds forever, for information nobody is reading.
  const agentStatus = useStationStatus(phase === "waiting" || phase === "refused");

  useEffect(() => {
    let cancelled = false;
    let timer: ReturnType<typeof setTimeout> | undefined;

    const attempt = async (n: number) => {
      try {
        const data = await apiGet<Me>("me");
        if (cancelled) return;
        setMe(data);
        // A real answer: trust it either way. Only a station passes; anything
        // else is a genuine refusal and must not keep retrying.
        setPhase(data?.profile?.kind === "station" ? "station" : "refused");
      } catch {
        if (cancelled) return;
        // No answer at all -- the agent has no token yet, or the network is
        // still coming up. Keep asking; the agent is doing the same.
        setPhase("waiting");
        timer = setTimeout(() => void attempt(n + 1), RETRY_MS[Math.min(n, RETRY_MS.length - 1)]);
      }
    };

    void attempt(0);
    return () => {
      cancelled = true;
      if (timer) clearTimeout(timer);
    };
  }, []);

  if (phase === "checking") return null;

  if (phase === "waiting") {
    // The spinner alone was the whole screen for the first year, and it was
    // shown identically for a three-second cold boot and for a PC that would
    // never connect again. Whatever the agent knows goes underneath it now:
    // a PC registered to another school, a clock two minutes out, a licence
    // that expired. `blocked` means retrying cannot help, so the spinner --
    // which promises that waiting will fix it -- comes off.
    const blocked = agentStatus?.phase === "blocked";
    return (
      <main className="mx-auto flex min-h-[60vh] max-w-2xl flex-col items-center justify-center gap-4 p-8 text-center">
        {blocked ? (
          <TriangleAlert aria-hidden="true" className="h-8 w-8 text-destructive" />
        ) : (
          <LoaderCircle aria-hidden="true" className="h-8 w-8 animate-spin text-accent" />
        )}
        {!blocked ? (
          <>
            <p className="text-lg font-semibold">{t("Station.connecting")}</p>
            <p className="max-w-md text-sm text-muted-foreground">{t("Station.connectingHint")}</p>
          </>
        ) : null}
        <StationDiagnostics status={agentStatus} />
      </main>
    );
  }

  if (phase === "refused") {
    return (
      <main className="mx-auto flex min-h-[60vh] max-w-2xl flex-col items-center justify-center gap-4 p-8 text-center">
        <p className="text-lg">{t("Station.notStation")}</p>
        <StationDiagnostics status={agentStatus} />
      </main>
    );
  }

  const stationName = me?.profile?.name ?? "";

  return (
    // pt-16 clears the fixed language/theme controls in the top-right corner
    // (see (kiosk)/kiosk-chrome.tsx), which would otherwise sit on the title.
    <main className="mx-auto max-w-6xl space-y-6 px-4 pb-10 pt-16 sm:space-y-8 sm:px-6 sm:pt-20">
      <header className="flex items-center gap-3 sm:gap-4">
        <BrandLogo
          size={56}
          className="h-11 w-11 shrink-0 rounded-2xl object-cover sm:h-14 sm:w-14"
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
