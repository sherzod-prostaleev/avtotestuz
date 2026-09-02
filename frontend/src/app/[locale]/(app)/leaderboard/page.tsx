"use client";

import { useCallback, useEffect, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import { apiGet } from "@/lib/api-client";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Award, RefreshCw, Trophy } from "lucide-react";
import { BackLink } from "@/components/layout/back-link";



export type LeaderboardPeriod = "daily" | "weekly" | "monthly" | "alltime";

export interface LeaderboardEntry {
  rank: number;
  name: string;
  score: number;
}

export interface LeaderboardYou {
  rank: number | null;
  score: number;
  name: string;
}

export interface LeaderboardResponse {
  period: LeaderboardPeriod;
  you: LeaderboardYou;
  top: LeaderboardEntry[];
  around_you: LeaderboardEntry[];
}

const PERIODS: {
  value: LeaderboardPeriod;
  labelKey: string;
  /** Phone-only label — four chips share 390px, so "Barcha vaqt" does not fit. */
  shortLabelKey?: string;
  /** Reads inside a sentence: "3 512 ball · bu hafta". */
  inPeriodKey: string;
}[] = [
  { value: "daily", labelKey: "periodDaily", inPeriodKey: "inPeriodDaily" },
  { value: "weekly", labelKey: "periodWeekly", inPeriodKey: "inPeriodWeekly" },
  { value: "monthly", labelKey: "periodMonthly", inPeriodKey: "inPeriodMonthly" },
  {
    value: "alltime",
    labelKey: "periodAlltime",
    shortLabelKey: "periodAlltimeShort",
    inPeriodKey: "inPeriodAlltime",
  },
];

/**
 * "4820" → "4 820". The phone rows put the score against the name in a 390px
 * row, where an unbroken five-digit number is hard to read at a glance.
 *
 * Done by hand rather than through `Intl.NumberFormat`: the app's locales are
 * `uz-Latn` / `uz-Cyrl`, which are not resolvable BCP-47 tags everywhere, and
 * a grouped number must not depend on which ICU data the browser shipped.
 */
function groupDigits(score: number): string {
  return String(score).replace(/\B(?=(\d{3})+(?!\d))/g, " ");
}

/** Medal tint for the top three phone rows: gold, silver, bronze. */
const MEDAL_TONE: Record<number, string> = {
  1: "text-gold",
  2: "text-muted-foreground",
  // No bronze design token exists; this is the artboard's value, used once.
  3: "text-[hsl(25_60%_48%)]",
};

/** Medal styling for the top 3 ranks; every other rank falls back to a plain badge. */
const RANK_BADGE_STYLES: Record<number, string> = {
  1: "bg-gold text-slate-950 border-amber-600",
  2: "bg-slate-300 text-slate-900 border-slate-400",
  3: "bg-amber-700/80 text-white border-amber-800",
};

function RankBadge({ rank }: { rank: number }) {
  const style = RANK_BADGE_STYLES[rank] ?? "bg-background text-muted-foreground border-border";
  return (
    <span
      className={`flex h-8 w-8 shrink-0 items-center justify-center rounded-full border text-xs font-extrabold ${style}`}
    >
      {rank}
    </span>
  );
}

function EntryRow({ entry, highlighted, youBadge }: { entry: LeaderboardEntry; highlighted: boolean; youBadge: string }) {
  return (
    <li
      className={`flex items-center gap-3 rounded-xl px-3 py-2.5 transition-colors ${
        highlighted ? "border border-accent bg-accent/10" : "border border-transparent"
      }`}
    >
      <RankBadge rank={entry.rank} />
      <span className="min-w-0 flex-1 truncate text-sm font-bold text-foreground">{entry.name}</span>
      {highlighted && (
        <span className="shrink-0 rounded-full bg-accent/20 px-2 py-0.5 text-[10px] font-extrabold text-accent">
          {youBadge}
        </span>
      )}
      <span className="shrink-0 font-display text-sm font-extrabold text-gold">{entry.score}</span>
    </li>
  );
}

/**
 * One 42px phone row. Ten of them plus the header, the period chips and the
 * position card come to 664px, which is exactly the content height a phone
 * leaves under the 60px top bar and the 4.25rem tab reserve — so the whole
 * ranking arrives on one screen with nothing to scroll.
 *
 * Not a link and not a button, so 42px is a reading row rather than a tap
 * target below the 44px minimum.
 */
function PhoneRow({
  entry,
  isYou,
  youBadge,
  first,
}: {
  entry: LeaderboardEntry;
  isYou: boolean;
  youBadge: string;
  first: boolean;
}) {
  return (
    <li
      // The separator is inset 48px to clear the rank column, which a border on
      // the row itself cannot do — hence the pseudo-element.
      className={`relative flex min-h-[42px] items-center gap-2.5 px-3 ${
        first ? "" : "before:absolute before:inset-x-0 before:left-12 before:top-0 before:h-px before:bg-border"
      } ${isYou ? "bg-accent/[0.08]" : ""}`}
    >
      <span className="flex w-[26px] shrink-0 justify-center">
        {entry.rank <= 3 ? (
          <Award aria-hidden="true" className={`h-[18px] w-[18px] ${MEDAL_TONE[entry.rank]}`} />
        ) : (
          <span className="text-sm font-bold tabular-nums text-muted-foreground">{entry.rank}</span>
        )}
      </span>
      <span className={`min-w-0 flex-1 truncate text-sm ${isYou ? "font-bold" : "font-semibold"}`}>
        {entry.name}
      </span>
      {isYou && (
        <span className="inline-flex h-5 shrink-0 items-center rounded-full bg-accent px-[7px] text-xs font-extrabold text-accent-foreground">
          {youBadge}
        </span>
      )}
      <span className="shrink-0 text-sm font-bold tabular-nums">{groupDigits(entry.score)}</span>
    </li>
  );
}

export interface LeaderboardPageProps {
  // Reused as-is under the login-free kiosk (frontend/src/app/[locale]/(kiosk)/station/leaderboard/page.tsx):
  // a station never appears in the rankings (leaderboard.Service.RecordPoint
  // returns early for a kind='station' profile), so this page has no
  // dashboard, premium, checkout or profile surface of its own to worry
  // about — the only navigation is the back link, which must stay inside
  // /station instead of the learner app's gated /dashboard.
  kiosk?: boolean;
}

export default function LeaderboardPage({ kiosk = false }: LeaderboardPageProps = {}) {
  const t = useTranslations("Leaderboard");
  const locale = useLocale();
  const backHref = kiosk ? `/${locale}/station` : `/${locale}/dashboard`;

  const [period, setPeriod] = useState<LeaderboardPeriod>("weekly");
  const currentPeriod = PERIODS.find((item) => item.value === period) ?? PERIODS[1];
  const [data, setData] = useState<LeaderboardResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);

  const load = useCallback(async (p: LeaderboardPeriod) => {
    setLoading(true);
    setError(false);
    try {
      const res = await apiGet<LeaderboardResponse>(`leaderboard?period=${p}`);
      setData(res);
    } catch {
      setError(true);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load(period);
  }, [load, period]);

  return (
    // `max-md:!pb-0`: the shell's own bottom padding sits on top of the tab
    // bar's 4.25rem reserve, and those 16px are the difference between ten rows
    // fitting and the tenth being clipped.
    <main className="page-shell-narrow max-md:!pb-0">
      {/* On a phone the bottom tab bar already says where you are and the menu
          already leads back, so the design drops the back link and the sentence
          of explanation to buy the rows their space. */}
      <header className="mb-6 max-md:mb-2">
        <div className="max-md:hidden">
          <BackLink href={backHref} kiosk={kiosk}>{t("backHome")}</BackLink>
        </div>
        <h1 className="flex items-center gap-2 font-display text-2xl font-bold tracking-tight max-md:leading-[1.15]">
          <Trophy aria-hidden="true" className="h-6 w-6 text-gold max-md:hidden" />
          {t("title")}
        </h1>
        <p className="text-sm text-muted-foreground max-md:hidden">{t("subtitle")}</p>
      </header>

      {/* Four chips on one row at every width. Below `sm` they used to wrap to
          2×2 and cost 100px; `sm:` and up is unchanged. */}
      <div
        role="radiogroup"
        aria-label={t("periodGroupLabel")}
        className="mb-6 grid grid-cols-4 gap-1.5 max-md:mb-2 sm:gap-2"
      >
        {PERIODS.map((item) => {
          const isSelected = period === item.value;
          return (
            <button
              key={item.value}
              type="button"
              role="radio"
              aria-checked={isSelected}
              onClick={() => setPeriod(item.value)}
              // The accessible name stays the full label at every width, so the
              // phone's shorter text cannot change what a test or a screen
              // reader asks for.
              aria-label={t(item.labelKey)}
              className={`filter-chip relative w-full transition-colors max-md:px-1 ${
                isSelected ? "font-extrabold text-accent-foreground" : "filter-chip-idle font-bold"
              }`}
            >
              {isSelected && (
                <span
                  aria-hidden="true"
                  className="nav-pill-in absolute inset-0 rounded-xl bg-accent shadow-3d"
                />
              )}
              <span className={`relative z-10 ${item.shortLabelKey ? "max-md:hidden" : ""}`}>
                {t(item.labelKey)}
              </span>
              {item.shortLabelKey && (
                <span className="relative z-10 md:hidden">{t(item.shortLabelKey)}</span>
              )}
            </button>
          );
        })}
      </div>


      {loading ? (
        <p role="status" className="py-12 text-center text-sm text-muted-foreground">
          {t("loading")}
        </p>
      ) : error ? (
        <div role="alert" className="rounded-xl border border-destructive/50 bg-destructive/10 p-4 text-sm text-destructive">
          <p>{t("loadError")}</p>
          <Button type="button" variant="outline" size="sm" className="mt-3" onClick={() => void load(period)}>
            <RefreshCw aria-hidden="true" className="mr-2 h-4 w-4" /> {t("retry")}
          </Button>
        </div>
      ) : data ? (
        <>
        {/* Phone: the design's one-screen ranking. A body of its own rather
            than a reflow of the wide one, because the wide layout — the back
            link, the roomy cards, the "around you" list — is also what the
            kiosk renders on a large screen and must not move. */}
        <div className="md:hidden">
          <div className="surface-raised mb-2 flex items-center gap-3 rounded-2xl border border-accent/40 bg-accent/[0.06] px-3.5 py-2">
            <span className="font-display text-[26px] font-extrabold leading-none text-accent">
              {data.you.rank === null ? t("rankNone") : t("rankValue", { rank: data.you.rank })}
            </span>
            <div className="min-w-0 flex-1">
              <p className="truncate text-sm font-bold leading-snug">{t("yourPositionLabel")}</p>
              <p className="truncate text-xs leading-snug text-muted-foreground">
                {t("yourScoreLine", {
                  score: groupDigits(data.you.score),
                  period: t(currentPeriod.inPeriodKey),
                })}
              </p>
            </div>
          </div>

          {data.you.rank === null && (
            <p className="mb-3 text-xs leading-snug text-muted-foreground">{t("notRankedYet")}</p>
          )}

          <p className="mb-1 text-xs font-extrabold uppercase leading-none tracking-[0.08em] text-muted-foreground">
            {t("topTitle")}
          </p>
          {data.top.length === 0 ? (
            <p className="rounded-2xl border border-border bg-card py-6 text-center text-sm text-muted-foreground">
              {t("topEmpty")}
            </p>
          ) : (
            <ol className="surface-raised-sm overflow-hidden rounded-2xl border border-border bg-card py-1">
              {data.top.map((entry, index) => (
                <PhoneRow
                  key={entry.rank}
                  entry={entry}
                  isYou={data.you.rank === entry.rank}
                  youBadge={t("youBadge")}
                  first={index === 0}
                />
              ))}
            </ol>
          )}
        </div>

        <div className="space-y-6 max-md:hidden">
          {/* Your rank card */}
          <Card className="overflow-hidden border-gold/40 bg-card p-5 sm:p-6">
            <p className="mb-3 text-xs font-bold uppercase tracking-wider text-muted-foreground">
              {t("yourPositionLabel")}
            </p>
            <div className="flex items-center gap-4">
              <span className="flex h-14 w-14 shrink-0 items-center justify-center rounded-2xl border border-gold/40 bg-gold/10 font-display text-xl font-extrabold text-gold">
                {data.you.rank === null ? t("rankNone") : t("rankValue", { rank: data.you.rank })}
              </span>
              <span className="min-w-0 flex-1 truncate font-display text-lg font-bold text-foreground">
                {data.you.name}
              </span>
              <span className="shrink-0 text-right">
                <span className="font-display text-2xl font-extrabold text-gold">{data.you.score}</span>
              </span>
            </div>
            {data.you.rank === null && (
              <p className="mt-4 text-xs text-muted-foreground">{t("notRankedYet")}</p>
            )}
          </Card>

          {/* Top 10 */}
          <Card className="p-6">
            <CardHeader className="p-0 mb-4">
              <CardTitle className="text-lg font-bold">{t("topTitle")}</CardTitle>
            </CardHeader>
            <CardContent className="p-0">
              {data.top.length === 0 ? (
                <p className="py-6 text-center text-sm text-muted-foreground">{t("topEmpty")}</p>
              ) : (
                <ol className="space-y-1">
                  {data.top.map((entry) => (
                    <EntryRow
                      key={entry.rank}
                      entry={entry}
                      highlighted={data.you.rank === entry.rank}
                      youBadge={t("youBadge")}
                    />
                  ))}
                </ol>
              )}
            </CardContent>
          </Card>

          {/* Around you — only when the caller falls outside the top 10 */}
          {data.around_you.length > 0 && (
            <Card className="p-6">
              <CardHeader className="p-0 mb-4">
                <CardTitle className="text-lg font-bold">{t("aroundYouTitle")}</CardTitle>
              </CardHeader>
              <CardContent className="p-0">
                <ol className="space-y-1">
                  {data.around_you.map((entry) => (
                    <EntryRow
                      key={entry.rank}
                      entry={entry}
                      highlighted={data.you.rank === entry.rank}
                      youBadge={t("youBadge")}
                    />
                  ))}
                </ol>
              </CardContent>
            </Card>
          )}
        </div>
        </>
      ) : null}
    </main>
  );
}
