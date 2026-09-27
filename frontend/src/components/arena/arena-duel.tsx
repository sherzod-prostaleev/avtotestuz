"use client";

import { useEffect, useState, type ReactNode } from "react";
import { useTranslations } from "next-intl";
import { Bot, Check, ChevronLeft, Clock, Flag, Trophy, X } from "lucide-react";
import type { ArenaMode } from "@/lib/arena-protocol";
import type { MatchView } from "@/lib/arena-state";
import { Avatar, RaceBoard, useMedalName } from "@/components/arena/arena-parts";

function elapsedLabel(totalSec: number): string {
  const safe = Math.max(0, totalSec);
  return `${Math.floor(safe / 60)}:${String(safe % 60).padStart(2, "0")}`;
}

function useModeLabel() {
  const t = useTranslations("Arena");
  return (mode: ArenaMode) =>
    mode === "bot" ? t("modeBot") : mode === "friend" ? t("modeFriend") : t("modeRanked");
}

export function ArenaSearching({
  elapsedSec,
  online,
  onCancel,
  onPracticeBot,
}: {
  elapsedSec: number;
  online: number | null;
  onCancel: () => void;
  onPracticeBot: () => void;
}) {
  const t = useTranslations("Arena");
  // After a while an empty arena is the likely story; offer the way out
  // instead of letting the player stare at the radar until the timeout.
  const suggestBot = elapsedSec >= 15;
  return (
    <div className="flex min-h-0 flex-1 flex-col gap-3">
      <div className="flex flex-none items-center gap-2.5">
        <button
          type="button"
          onClick={onCancel}
          aria-label={t("cancelSearch")}
          className="-ml-1 flex h-11 w-11 flex-none items-center justify-center text-muted-foreground"
        >
          <ChevronLeft aria-hidden="true" className="h-[22px] w-[22px]" />
        </button>
        <h1 className="font-display text-xl font-extrabold">{t("title")}</h1>
      </div>

      <div
        className="flex min-h-0 flex-1 flex-col items-center justify-center gap-5 text-center"
        role="status"
        aria-live="polite"
      >
        <div className="relative flex h-[148px] w-[148px] items-center justify-center">
          <span aria-hidden="true" className="absolute inset-0 rounded-full border-2 border-accent/20 motion-safe:animate-ping [animation-duration:2.2s]" />
          <span aria-hidden="true" className="absolute inset-3 rounded-full border-2 border-accent/25" />
          <span aria-hidden="true" className="absolute inset-8 rounded-full border-2 border-accent/40" />
          <span className="flex h-14 w-14 items-center justify-center rounded-full bg-accent/15">
            <span
              aria-hidden="true"
              className="h-7 w-7 rounded-full border-[3px] border-accent border-t-transparent motion-safe:animate-spin"
            />
          </span>
        </div>
        <div>
          <p className="font-display text-xl font-extrabold">{t("searching")}</p>
          <p className="mt-1 text-sm text-muted-foreground">{t("searchingHint")}</p>
          <p className="mt-2 font-display text-lg font-bold tabular-nums text-accent">
            {elapsedLabel(elapsedSec)}
          </p>
          {online !== null && (
            <p className="mt-1 text-xs text-muted-foreground">{t("onlineNow", { count: online })}</p>
          )}
        </div>
        {suggestBot && (
          <div className="w-full max-w-sm rounded-xl border border-border bg-card p-3 text-sm">
            <p className="text-muted-foreground">{t("searchFewPlayers")}</p>
            <button
              type="button"
              onClick={onPracticeBot}
              className="mt-2 inline-flex min-h-11 items-center gap-2 rounded-xl border border-border px-4 font-bold"
            >
              <Bot aria-hidden="true" className="h-4 w-4 text-accent" />
              {t("practiceBot")}
            </button>
          </div>
        )}
      </div>

      <button
        type="button"
        onClick={onCancel}
        className="mb-3 min-h-[50px] w-full flex-none rounded-xl border border-border text-base font-bold text-muted-foreground"
      >
        {t("cancelSearch")}
      </button>
    </div>
  );
}

/** The server's pre-duel countdown (protocol.go Countdown). The first render
 *  can see a stale clock, so the number shown never exceeds it. */
const COUNTDOWN_SEC = 3;

export function ArenaCountdown({ match, secondsLeft }: { match: MatchView; secondsLeft: number }) {
  const t = useTranslations("Arena");
  const medalName = useMedalName();
  const modeLabel = useModeLabel();
  const opp = match.opponent;
  const oppName = opp.bot ? t("botName") : opp.name;
  const player = (name: string, side: "you" | "opponent", rating: number, medal: string, bot?: boolean) => (
    <div className="flex min-w-0 flex-1 flex-col items-center gap-2 text-center">
      <Avatar name={name} side={side} bot={bot} size="lg" />
      <p className="w-full truncate font-display text-lg font-extrabold">{name}</p>
      <p className="inline-flex items-center gap-1 text-xs font-bold text-muted-foreground">
        <Trophy aria-hidden="true" className="h-3.5 w-3.5 text-gold" />
        <span className="tabular-nums">{rating}</span> · {medalName(medal)}
      </p>
    </div>
  );
  return (
    <div className="flex min-h-0 flex-1 flex-col items-center justify-center gap-6 text-center" role="status" aria-live="polite">
      <span className="rounded-full border border-border bg-card px-3 py-1 text-xs font-extrabold uppercase tracking-[0.08em] text-muted-foreground">
        {modeLabel(match.mode)}
      </span>
      <div className="flex w-full max-w-md items-center gap-3">
        {player(match.you.name || t("you"), "you", match.you.rating, match.you.medal)}
        <span className="font-display text-3xl font-extrabold italic text-accent">{t("vs")}</span>
        {player(oppName, "opponent", opp.rating, opp.medal, opp.bot)}
      </div>
      <div>
        <p className="text-sm text-muted-foreground">{t("startingIn")}</p>
        <p key={secondsLeft} className="font-display text-6xl font-extrabold tabular-nums text-accent motion-safe:animate-[fadeIn_0.3s_ease-out]">
          {Math.min(COUNTDOWN_SEC, Math.max(1, secondsLeft))}
        </p>
      </div>
      <p className="max-w-sm text-xs text-muted-foreground">{t("rule2")}</p>
    </div>
  );
}

export interface ArenaQuestionProps {
  match: MatchView;
  phase: "question" | "reveal";
  secondsLeft: number;
  onAnswer: (answerId: string) => void;
  onForfeit: () => void;
}

const LETTERS = ["A", "B", "C", "D", "E", "F"];

/**
 * The question clock as a bar. It drains with a CSS animation started at the
 * right point (negative delay = time already used, measured once at mount),
 * so the bar moves smoothly without a React render per frame.
 */
function DrainBar({ deadline, windowMs, urgent }: { deadline: number; windowMs: number; urgent: boolean }) {
  const [usedMs] = useState(() => Math.min(windowMs, Math.max(0, windowMs - (deadline - Date.now()))));
  return (
    <span
      className={`arena-drain block h-full origin-left ${urgent ? "bg-danger" : "bg-accent"}`}
      style={{ animationDuration: `${windowMs}ms`, animationDelay: `-${usedMs}ms` }}
    />
  );
}

export function ArenaQuestion({ match, phase, secondsLeft, onAnswer, onForfeit }: ArenaQuestionProps) {
  const t = useTranslations("Arena");
  const q = match.question;
  const verdict = phase === "reveal" ? match.verdict : null;
  const locked = phase !== "question" || match.pending;

  // Digits 1–6 answer on a keyboard, the way the answer letters read.
  useEffect(() => {
    if (phase !== "question" || !q) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.altKey || e.ctrlKey || e.metaKey) return;
      const target = e.target as HTMLElement | null;
      if (target && (target.tagName === "INPUT" || target.tagName === "TEXTAREA")) return;
      const n = Number(e.key);
      if (Number.isInteger(n) && n >= 1 && n <= q.answers.length) {
        onAnswer(q.answers[n - 1].id);
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [phase, q, onAnswer]);

  if (!q) return null;
  const windowMs = match.questionTimeMs || 15000;
  const urgent = phase === "question" && secondsLeft <= 5;

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-2.5">
      <RaceBoard
        you={match.you}
        opponent={match.opponent}
        total={match.total}
        marks={match.marks}
        current={match.index}
        opponentAnswered={match.opponent.answered}
        opponentFinished={match.opponent.finished}
        opponentOffline={!match.opponent.connected}
      />

      {/* One row carries the counter — or, during the reveal, the verdict —
          plus the clock and the forfeit flag, so a phone keeps its height
          for the question and its answers. */}
      <div className="flex min-h-9 flex-none items-center gap-2" aria-live="polite">
        {verdict ? (
          <p
            className={`min-w-0 flex-1 truncate font-display text-base font-extrabold ${
              verdict.correct ? "text-success" : verdict.answered ? "text-danger" : "text-muted-foreground"
            }`}
          >
            {verdict.correct
              ? t("verdictCorrect")
              : verdict.answered
                ? t("verdictWrong")
                : t("verdictTimeout")}
            {verdict.points > 0 && (
              <span className="ml-2 text-sm">{t("verdictPoints", { points: verdict.points })}</span>
            )}
          </p>
        ) : (
          <span className="min-w-0 flex-1 truncate text-xs font-extrabold uppercase tracking-[0.06em] text-muted-foreground">
            {t("questionOf", { current: match.index + 1, total: match.total })}
          </span>
        )}
        <span
          className={`inline-flex flex-none items-center gap-1.5 font-display text-lg font-extrabold tabular-nums ${
            urgent ? "text-danger" : "text-accent"
          }`}
          aria-label={t("secondsLeft", { seconds: secondsLeft })}
        >
          <Clock aria-hidden="true" className="h-4 w-4" />
          {phase === "question" ? String(secondsLeft).padStart(2, "0") : "--"}
        </span>
        <button
          type="button"
          onClick={onForfeit}
          aria-label={t("forfeit")}
          title={t("forfeit")}
          className="-mr-1 flex h-9 w-9 flex-none items-center justify-center rounded-lg text-muted-foreground hover:bg-danger/10 hover:text-danger"
        >
          <Flag aria-hidden="true" className="h-4 w-4" />
        </button>
      </div>

      <span aria-hidden="true" className="block h-[5px] flex-none overflow-hidden rounded-full bg-border">
        {phase === "question" && (
          <DrainBar
            key={`${match.id}-${match.index}`}
            deadline={match.deadline}
            windowMs={windowMs}
            urgent={urgent}
          />
        )}
      </span>

      <div className="flex min-h-0 flex-1 flex-col gap-2.5 overflow-y-auto overscroll-contain">
        <p className="mt-1 text-base font-bold leading-[1.45]">{q.text}</p>
        {q.image_url && (
          // eslint-disable-next-line @next/next/no-img-element
          <img
            src={q.image_url}
            alt=""
            className="max-h-[21dvh] w-full flex-none rounded-xl bg-muted/40 object-contain md:max-h-64"
          />
        )}
        <ul className="flex flex-col gap-2">
          {q.answers.map((a, i) => {
            const isSel = match.selected === a.id;
            const isCorrect = verdict?.correct_answer_id === a.id;
            let box = "border-border bg-card hover:border-accent";
            let badge = "border-border text-muted-foreground";
            let icon: ReactNode = LETTERS[i] ?? String(i + 1);
            if (verdict && isCorrect) {
              box = "border-success bg-success/15";
              badge = "border-success bg-success text-white";
              icon = <Check aria-hidden="true" className="h-4 w-4" strokeWidth={3} />;
            } else if (verdict && isSel) {
              box = "border-danger bg-danger/15";
              badge = "border-danger bg-danger text-white";
              icon = <X aria-hidden="true" className="h-4 w-4" strokeWidth={3} />;
            } else if (isSel) {
              box = "border-accent bg-accent/[0.12]";
              badge = "border-accent bg-accent text-accent-foreground";
            } else if (verdict) {
              box = "border-border bg-card opacity-60";
            }
            return (
              <li key={a.id}>
                <button
                  type="button"
                  disabled={locked}
                  aria-pressed={isSel}
                  onClick={() => onAnswer(a.id)}
                  className={`flex w-full min-h-[52px] items-center gap-3 rounded-xl border px-3 py-2 text-left transition-colors disabled:cursor-default ${box}`}
                >
                  <span
                    className={`flex h-7 w-7 flex-none items-center justify-center rounded-lg border-2 font-display text-sm font-extrabold ${badge}`}
                  >
                    {icon}
                  </span>
                  <span className="min-w-0 flex-1 text-sm leading-5">
                    {a.text}
                    {a.image_url && (
                      // eslint-disable-next-line @next/next/no-img-element
                      <img src={a.image_url} alt="" className="mt-1.5 max-h-24 rounded-lg object-contain" />
                    )}
                  </span>
                </button>
              </li>
            );
          })}
        </ul>
      </div>

    </div>
  );
}

export function ArenaWaiting({ match, onForfeit }: { match: MatchView; onForfeit: () => void }) {
  const t = useTranslations("Arena");
  const correct = match.marks.filter((m) => m === "correct").length;
  const oppName = match.opponent.bot ? t("botName") : match.opponent.name;
  return (
    <div className="flex min-h-0 flex-1 flex-col gap-4">
      <RaceBoard
        you={match.you}
        opponent={match.opponent}
        total={match.total}
        marks={match.marks}
        opponentAnswered={match.opponent.answered}
        opponentFinished={match.opponent.finished}
        opponentOffline={!match.opponent.connected}
        youFinished
      />
      <div className="flex min-h-0 flex-1 flex-col items-center justify-center gap-4 text-center" role="status" aria-live="polite">
        <span className="flex h-20 w-20 items-center justify-center rounded-full bg-success/15 text-success">
          <Check aria-hidden="true" className="h-10 w-10" strokeWidth={3} />
        </span>
        <div>
          <p className="font-display text-2xl font-extrabold">{t("waitingTitle")}</p>
          <p className="mt-1 inline-flex items-center gap-2 text-sm text-muted-foreground">
            <span
              aria-hidden="true"
              className="h-3.5 w-3.5 rounded-full border-2 border-gold border-t-transparent motion-safe:animate-spin"
            />
            {t("waitingBody", { name: oppName })}
          </p>
        </div>
        <div className="rounded-xl border border-border bg-card px-4 py-3">
          <p className="text-xs font-bold uppercase tracking-[0.08em] text-muted-foreground">
            {t("waitingYourResult")}
          </p>
          <p className="mt-1 font-display text-xl font-extrabold">
            {t("correctOf", { correct, total: match.total })}
          </p>
        </div>
      </div>
      <button
        type="button"
        onClick={onForfeit}
        className="mb-2 inline-flex min-h-11 flex-none items-center justify-center gap-1.5 text-sm font-semibold text-muted-foreground"
      >
        <Flag aria-hidden="true" className="h-4 w-4" />
        {t("forfeit")}
      </button>
    </div>
  );
}
