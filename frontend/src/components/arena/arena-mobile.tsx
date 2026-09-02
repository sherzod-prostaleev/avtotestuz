"use client";

import { useTranslations } from "next-intl";
import { ArrowRight, ChevronLeft, ChevronRight, Clock, Trophy, Users } from "lucide-react";
import type { ArenaPhase, QuestionPayload } from "@/lib/arena-protocol";

export type ArenaScorePair = { you: number; opponent: number };

export type ArenaHistoryItem = {
  match_id: string;
  outcome: string | null;
  score: number;
  rating_delta: number | null;
  medal: string;
  finished_at: string | null;
};

export type ArenaResultSummary = {
  outcome: string;
  reason: string;
  score: ArenaScorePair;
  correct: ArenaScorePair;
  ratingDelta?: number;
  medal?: string;
};

export interface ArenaMobileProps {
  phase: ArenaPhase;
  statusMsg: string;
  userName: string;
  opponentName: string;
  rating: { rating: number; medal: string } | null;
  history: ArenaHistoryItem[];
  inviteCode: string;
  joinCode: string;
  onJoinCodeChange: (value: string) => void;
  question: QuestionPayload | null;
  qIndex: number;
  qTotal: number;
  score: ArenaScorePair;
  selected: string | null;
  revealCorrect: string | null;
  acked: boolean;
  remaining: number;
  /** 0…1 — share of the question window still left, drives the bar width. */
  timerFraction: number;
  searchElapsedSec: number;
  result: ArenaResultSummary | null;
  /** Rating as it stood before this duel, for the result screen's before → after. */
  ratingBefore: number | null;
  onFindMatch: () => void;
  onCancelSearch: () => void;
  onCreateInvite: () => void;
  onJoinInvite: () => void;
  onAnswer: (answerId: string) => void;
  onForfeit: () => void;
  onPlayAgain: () => void;
  onBackToLobby: () => void;
  className?: string;
}

/** m:ss — the search screen counts in seconds and never reaches an hour. */
function elapsedLabel(totalSec: number): string {
  const safe = Math.max(0, totalSec);
  return `${Math.floor(safe / 60)}:${String(safe % 60).padStart(2, "0")}`;
}

function initial(name: string): string {
  return name.trim().charAt(0).toUpperCase();
}

function signed(value: number): string {
  return value > 0 ? `+${value}` : String(value);
}

export function ArenaMobile({
  phase,
  statusMsg,
  userName,
  opponentName,
  rating,
  history,
  inviteCode,
  joinCode,
  onJoinCodeChange,
  question,
  qIndex,
  qTotal,
  score,
  selected,
  revealCorrect,
  acked,
  remaining,
  timerFraction,
  searchElapsedSec,
  result,
  ratingBefore,
  onFindMatch,
  onCancelSearch,
  onCreateInvite,
  onJoinInvite,
  onAnswer,
  onForfeit,
  onPlayAgain,
  onBackToLobby,
  className = "",
}: ArenaMobileProps) {
  const t = useTranslations("Arena");

  /**
   * The medal comes off the wire as an English identifier and the design writes
   * it in Uzbek. Every key is a literal: `t(\`medal${x}\`)` built from server
   * text would be untranslatable to a bundler and unverifiable to a reader.
   * `medalLabel()` in arena-protocol stays exactly as it is — it is what the
   * ≥768px layout renders, and a change there would reflow the desktop chip.
   */
  function medalName(medal: string): string {
    switch (medal) {
      case "brilliant":
        return t("medalBrilliant");
      case "diamond":
        return t("medalDiamond");
      case "platinum":
        return t("medalPlatinum");
      case "gold":
        return t("medalGold");
      case "silver":
        return t("medalSilver");
      default:
        return t("medalBronze");
    }
  }

  function outcomeName(outcome: string | null): string {
    if (outcome === "won") return t("outcomeWon");
    if (outcome === "lost") return t("outcomeLost");
    return t("draw");
  }

  const isLobby =
    phase === "idle" || phase === "connecting" || phase === "lobby" || phase === "error";
  const isDuel = phase === "question" || phase === "reveal";
  const isSearch = phase === "searching" || phase === "countdown";

  // The lobby is 456px tall and may grow with a longer locale, so it is allowed
  // to scroll. Every other state is drawn as exactly one screen, and gets the
  // height the shell leaves between the top bar and the tab reserve.
  const fit = isLobby ? "" : "mobile-fit-screen min-h-0";
  const gap = isDuel ? "gap-2.5" : "gap-3";

  return (
    <div
      data-testid="arena-mobile"
      // `md:[&+*]:!mt-0`: a `md:hidden` element still counts as a child, so
      // without it the wide body below would inherit a margin it never had.
      className={`md:hidden md:[&+*]:!mt-0 flex flex-col px-3 pt-3 ${gap} ${fit} ${className}`}
    >
      {isLobby && (
        <>
          <div className="flex items-start justify-between gap-2.5">
            <div className="min-w-0">
              <h1 className="font-display text-2xl font-extrabold leading-[1.15] tracking-tight">
                {t("title")}
              </h1>
              <p className="text-sm leading-[21px] text-muted-foreground">{t("subtitleShort")}</p>
            </div>
            {rating ? (
              <div className="flex h-11 flex-none items-center gap-[7px] rounded-xl border border-border bg-card px-3">
                <Trophy aria-hidden="true" className="h-[17px] w-[17px] text-gold" />
                <span className="font-display text-base font-extrabold tabular-nums">
                  {rating.rating}
                </span>
                <span className="text-xs text-muted-foreground">{medalName(rating.medal)}</span>
              </div>
            ) : (
              // Never a guessed number while `me/arena/rating` is in flight.
              <span
                aria-hidden="true"
                className="h-11 w-[122px] flex-none animate-pulse rounded-xl bg-card"
              />
            )}
          </div>

          <button
            type="button"
            onClick={onFindMatch}
            disabled={phase === "connecting"}
            className="btn-3d-primary flex min-h-[50px] w-full items-center justify-center gap-2 rounded-xl font-display text-lg font-extrabold disabled:opacity-60"
          >
            <Users aria-hidden="true" className="h-5 w-5" strokeWidth={2.5} />
            {t("findMatch")}
          </button>

          {statusMsg && (
            <p className="rounded-xl border border-border bg-muted/40 px-3 py-2 text-xs">
              {statusMsg}
            </p>
          )}

          <div className="surface-raised flex flex-col gap-2.5 rounded-xl border border-border bg-card px-3.5 py-3">
            <div>
              <p className="text-base font-bold">{t("inviteTitle")}</p>
              <p className="mt-px text-xs text-muted-foreground">{t("inviteHint")}</p>
            </div>
            <div className="flex gap-2">
              <button
                type="button"
                onClick={onCreateInvite}
                className="min-h-touch flex-none rounded-xl border border-border px-3.5 text-sm font-bold"
              >
                {t("createInvite")}
              </button>
              <input
                value={joinCode}
                onChange={(e) => onJoinCodeChange(e.target.value)}
                placeholder={t("invitePlaceholder")}
                maxLength={12}
                className="min-h-touch min-w-0 flex-1 rounded-xl border border-border bg-background px-3 text-sm font-bold uppercase tracking-[0.18em] text-foreground"
              />
              <button
                type="button"
                onClick={onJoinInvite}
                aria-label={t("joinInvite")}
                className="btn-3d-primary flex h-11 w-11 min-h-touch flex-none items-center justify-center rounded-xl"
              >
                <ChevronRight aria-hidden="true" className="h-5 w-5" strokeWidth={2.5} />
              </button>
            </div>
            {inviteCode && (
              // The design's row has no room for the chip, and the code is the
              // one thing the friend has to be told — so it goes underneath.
              <span className="font-mono text-sm font-extrabold tracking-[0.18em] text-accent">
                {inviteCode}
              </span>
            )}
          </div>

          {history.length > 0 && (
            <div>
              <p className="mb-[5px] text-xs font-extrabold uppercase tracking-[0.08em] text-muted-foreground">
                {t("historyTitle")}
              </p>
              <ul className="overflow-hidden rounded-xl border border-border bg-card">
                {history.slice(0, 3).map((item, index) => {
                  const won = item.outcome === "won";
                  const lost = item.outcome === "lost";
                  const dot = won ? "bg-success" : lost ? "bg-danger" : "bg-muted-foreground";
                  const tone = won
                    ? "text-success"
                    : lost
                      ? "text-danger"
                      : "text-muted-foreground";
                  const delta = item.rating_delta ?? 0;
                  return (
                    <li key={item.match_id}>
                      {index > 0 && <div aria-hidden="true" className="ml-[34px] h-px bg-border" />}
                      <div className="flex min-h-12 items-center gap-3 px-3.5">
                        <span aria-hidden="true" className={`h-2 w-2 flex-none rounded-full ${dot}`} />
                        <span className="min-w-0 flex-1 truncate text-sm font-semibold">
                          {outcomeName(item.outcome)}
                        </span>
                        <span className={`text-sm font-bold tabular-nums ${tone}`}>
                          {t("historyScore", { score: item.score })}
                        </span>
                        <span
                          className={`w-[38px] flex-none text-right text-xs font-bold tabular-nums ${tone}`}
                        >
                          {signed(delta)}
                        </span>
                      </div>
                    </li>
                  );
                })}
              </ul>
            </div>
          )}
        </>
      )}

      {isSearch && (
        <>
          <div className="flex flex-none items-center gap-2.5">
            <button
              type="button"
              onClick={onCancelSearch}
              aria-label={t("cancelSearch")}
              className="-ml-1 flex h-11 w-11 flex-none items-center justify-center text-muted-foreground"
            >
              <ChevronLeft aria-hidden="true" className="h-[22px] w-[22px]" />
            </button>
            <h1 className="font-display text-xl font-extrabold">{t("title")}</h1>
          </div>

          <div className="flex min-h-0 flex-1 flex-col items-center justify-center gap-[18px] text-center">
            <div className="relative flex h-[132px] w-[132px] items-center justify-center">
              <span
                aria-hidden="true"
                className={`absolute inset-0 rounded-full border-2 border-accent/20 ${
                  phase === "searching" ? "motion-safe:animate-pulse" : ""
                }`}
              />
              <span aria-hidden="true" className="absolute inset-4 rounded-full border-2 border-accent/30" />
              <span aria-hidden="true" className="absolute inset-8 rounded-full border-2 border-accent/50" />
              <span className="flex h-14 w-14 items-center justify-center rounded-full bg-muted font-display text-xl font-extrabold text-accent">
                {phase === "countdown" ? initial(opponentName) : initial(userName)}
              </span>
            </div>
            {phase === "countdown" ? (
              <div>
                <p className="text-sm text-muted-foreground">{t("vs")}</p>
                <p className="mt-1 font-display text-2xl font-extrabold">{opponentName}</p>
                <p className="mt-1 text-sm font-bold text-accent">{t("starting")}</p>
              </div>
            ) : (
              <div>
                <p className="font-display text-xl font-extrabold">{t("searching")}</p>
                <p className="mt-1 text-sm tabular-nums text-muted-foreground">
                  {elapsedLabel(searchElapsedSec)}
                </p>
              </div>
            )}
          </div>

          <button
            type="button"
            onClick={onCancelSearch}
            className="mb-3 min-h-[50px] w-full flex-none rounded-xl border border-border text-base font-bold text-muted-foreground"
          >
            {t("cancelSearch")}
          </button>
        </>
      )}

      {isDuel && question && question.answers.length > 0 && (
        <>
          <div className="flex flex-none items-center justify-between gap-2.5">
            <span className="text-xs font-extrabold uppercase tracking-[0.06em] text-muted-foreground">
              {t("questionProgress", { current: qIndex + 1, total: qTotal })}
            </span>
            <span className="inline-flex items-center gap-1.5 font-display text-base font-extrabold tabular-nums text-accent">
              <Clock aria-hidden="true" className="h-4 w-4" />
              {String(remaining).padStart(2, "0")}
            </span>
          </div>

          {/* Width is inline: a Tailwind class built at runtime never reaches
              the stylesheet. */}
          <span
            aria-hidden="true"
            className="block h-[5px] flex-none overflow-hidden rounded-full bg-border"
          >
            <span
              className="block h-full bg-accent"
              style={{ width: `${Math.round(Math.max(0, Math.min(1, timerFraction)) * 100)}%` }}
            />
          </span>

          <div className="flex flex-none items-center gap-2.5 rounded-xl border border-border bg-card px-3 py-2">
            <span className="flex h-[30px] w-[30px] flex-none items-center justify-center rounded-full bg-accent/20 font-display text-sm font-extrabold text-accent">
              {initial(userName)}
            </span>
            <span className="text-sm font-bold">{t("you")}</span>
            <span className="min-w-0 flex-1 text-center font-display text-xl font-extrabold tabular-nums">
              {score.you} : {score.opponent}
            </span>
            <span className="max-w-[92px] truncate text-sm font-bold text-muted-foreground">
              {opponentName}
            </span>
            <span className="flex h-[30px] w-[30px] flex-none items-center justify-center rounded-full bg-muted font-display text-sm font-extrabold text-muted-foreground">
              {initial(opponentName)}
            </span>
          </div>

          {/* The question and its answers scroll; the forfeit control below
              stays on screen, which is what the design pins to the bottom. */}
          <div className="flex min-h-0 flex-1 flex-col gap-2.5 overflow-y-auto">
            <p className="mt-1 text-base font-bold leading-[23px]">{question.text}</p>
            {question.image_url && (
              // eslint-disable-next-line @next/next/no-img-element
              <img
                src={question.image_url}
                alt=""
                className="max-h-[22dvh] w-full flex-none rounded-xl object-contain"
              />
            )}
            <ul className="flex flex-col gap-2">
              {question.answers.map((a) => {
                const isSel = selected === a.id;
                const isCorrect = revealCorrect === a.id;
                const showReveal = phase === "reveal";
                // Same order as the wide layout, so reveal reads identically.
                let box = "border-border bg-card";
                let ring = "border-muted-foreground/60";
                let dot = "";
                if (showReveal && isCorrect) {
                  box = "border-success bg-success/15";
                  ring = "border-success";
                  dot = "bg-success";
                } else if (showReveal && isSel && !isCorrect) {
                  box = "border-danger bg-danger/15";
                  ring = "border-danger";
                  dot = "bg-danger";
                } else if (isSel) {
                  box = "border-accent bg-accent/[0.12]";
                  ring = "border-accent";
                  dot = "bg-accent";
                }
                return (
                  <li key={a.id}>
                    <button
                      type="button"
                      disabled={acked || phase !== "question"}
                      onClick={() => onAnswer(a.id)}
                      className={`flex w-full min-h-[52px] items-center gap-2.5 rounded-xl border px-3 py-2 text-left transition-colors ${box}`}
                    >
                      <span
                        aria-hidden="true"
                        className={`flex h-[18px] w-[18px] flex-none items-center justify-center rounded-full border-2 ${ring}`}
                      >
                        {dot && <span className={`h-2 w-2 rounded-full ${dot}`} />}
                      </span>
                      <span className="min-w-0 flex-1 text-sm leading-5">{a.text}</span>
                    </button>
                  </li>
                );
              })}
            </ul>
          </div>

          <button
            type="button"
            onClick={onForfeit}
            className="mb-2 min-h-touch w-full flex-none text-sm font-semibold text-muted-foreground"
          >
            {t("forfeit")}
          </button>
        </>
      )}

      {phase === "result" && result && (
        <>
          <div className="flex min-h-0 flex-1 flex-col items-center justify-center gap-3.5 text-center">
            <span
              aria-hidden="true"
              className={`flex h-[84px] w-[84px] flex-none items-center justify-center rounded-full ${
                result.outcome === "won"
                  ? "bg-success/15 text-success"
                  : result.outcome === "lost"
                    ? "bg-danger/15 text-danger"
                    : "bg-muted text-muted-foreground"
              }`}
            >
              <Trophy className="h-10 w-10" />
            </span>
            <div>
              <p
                className={`font-display text-2xl font-extrabold ${
                  result.outcome === "won"
                    ? "text-success"
                    : result.outcome === "lost"
                      ? "text-danger"
                      : "text-muted-foreground"
                }`}
              >
                {result.outcome === "won"
                  ? t("youWonShort")
                  : result.outcome === "lost"
                    ? t("youLost")
                    : t("draw")}
              </p>
              <p className="mt-1.5 font-display text-[38px] font-extrabold leading-none tabular-nums">
                {result.score.you} : {result.score.opponent}
              </p>
              <p className="mt-1.5 text-sm text-muted-foreground">
                {t("correctLine", { you: result.correct.you, opponent: result.correct.opponent })}
              </p>
            </div>

            {ratingBefore != null && typeof result.ratingDelta === "number" && (
              <div className="flex flex-wrap items-center justify-center gap-2.5 rounded-xl border border-border bg-card px-3.5 py-2.5">
                <span className="text-sm text-muted-foreground">{t("ratingLabel")}</span>
                <span className="font-display text-base font-bold tabular-nums text-muted-foreground">
                  {ratingBefore}
                </span>
                <ArrowRight aria-hidden="true" className="h-4 w-4 text-muted-foreground" />
                <span className="font-display text-lg font-extrabold tabular-nums">
                  {ratingBefore + result.ratingDelta}
                </span>
                <span
                  className={`text-sm font-extrabold tabular-nums ${
                    result.ratingDelta >= 0 ? "text-success" : "text-danger"
                  }`}
                >
                  {signed(result.ratingDelta)}
                </span>
                {result.medal && (
                  <span className="text-xs text-muted-foreground">{medalName(result.medal)}</span>
                )}
              </div>
            )}
          </div>

          <div className="mb-3 flex flex-none flex-col gap-2">
            <button
              type="button"
              onClick={onPlayAgain}
              className="btn-3d-primary flex min-h-[50px] w-full items-center justify-center rounded-xl font-display text-lg font-extrabold"
            >
              {t("playAgain")}
            </button>
            <button
              type="button"
              onClick={onBackToLobby}
              className="min-h-[46px] w-full rounded-xl border border-border text-sm font-bold text-muted-foreground"
            >
              {t("backToLobbyFull")}
            </button>
          </div>
        </>
      )}
    </div>
  );
}
