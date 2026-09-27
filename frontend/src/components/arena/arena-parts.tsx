"use client";

import { useTranslations } from "next-intl";
import { Bot, Check, WifiOff } from "lucide-react";
import type { RoundMark } from "@/lib/arena-protocol";

export function initialOf(name: string): string {
  return name.trim().charAt(0).toUpperCase() || "?";
}

export function signed(value: number): string {
  return value > 0 ? `+${value}` : String(value);
}

/** Medal identifiers come off the wire in English; every key is a literal. */
export function useMedalName() {
  const t = useTranslations("Arena");
  return (medal: string): string => {
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
  };
}

type Side = "you" | "opponent";

export function Avatar({
  name,
  side,
  bot = false,
  size = "md",
}: {
  name: string;
  side: Side;
  bot?: boolean;
  size?: "sm" | "md" | "lg";
}) {
  const dims =
    size === "lg"
      ? "h-16 w-16 text-2xl"
      : size === "sm"
        ? "h-8 w-8 text-sm"
        : "h-10 w-10 text-base";
  const tone =
    side === "you"
      ? "bg-accent/20 text-accent ring-accent/40"
      : "bg-gold/20 text-gold ring-gold/40";
  return (
    <span
      aria-hidden="true"
      className={`flex flex-none items-center justify-center rounded-full font-display font-extrabold ring-2 ${dims} ${tone}`}
    >
      {bot ? <Bot className="h-1/2 w-1/2" strokeWidth={2.4} /> : initialOf(name)}
    </span>
  );
}

const MARK_CELL: Record<RoundMark, string> = {
  correct: "bg-success",
  wrong: "bg-danger",
  skipped: "bg-muted-foreground/50",
};

/**
 * One player's lane in the race: `total` cells that fill left to right, so
 * the lane reads as a bar filling up rather than as a number. Your own lane
 * colours each cell by its verdict; the opponent's lane only shows how far
 * they are — which of their answers were right stays hidden until the end.
 */
export function RaceLane({
  side,
  name,
  bot = false,
  total,
  marks,
  answered,
  current,
  finished = false,
  offline = false,
  label,
}: {
  side: Side;
  name: string;
  bot?: boolean;
  total: number;
  /** Your lane: verdicts so far. */
  marks?: RoundMark[];
  /** Opponent lane: how many questions they have got through. */
  answered?: number;
  /** Index of the question being played now (your lane only). */
  current?: number;
  finished?: boolean;
  offline?: boolean;
  label: string;
}) {
  const t = useTranslations("Arena");
  const done = side === "you" ? (marks?.length ?? 0) : Math.min(answered ?? 0, total);
  const pct = total > 0 ? Math.round((done / total) * 100) : 0;
  return (
    <div className="flex items-center gap-2">
      <Avatar name={name} side={side} bot={bot} size="sm" />
      <div className="flex w-[84px] flex-none flex-col sm:w-[128px]">
        <span className="truncate text-sm font-bold leading-tight">{name}</span>
        {offline ? (
          <span className="inline-flex items-center gap-1 text-[11px] font-bold leading-tight text-danger">
            <WifiOff aria-hidden="true" className="h-3 w-3" />
            {t("opponentOffline")}
          </span>
        ) : finished ? (
          <span className="inline-flex items-center gap-1 text-[11px] font-bold leading-tight text-success">
            <Check aria-hidden="true" className="h-3 w-3" strokeWidth={3} />
            {t("opponentFinished")}
          </span>
        ) : null}
      </div>
      <div
        role="progressbar"
        aria-label={label}
        aria-valuemin={0}
        aria-valuemax={total}
        aria-valuenow={done}
        aria-valuetext={`${pct}%`}
        className="flex h-2.5 min-w-0 flex-1 gap-[3px]"
      >
        {Array.from({ length: total }, (_, i) => {
          const filled = i < done;
          const cell =
            side === "you"
              ? filled
                ? MARK_CELL[marks![i] ?? "skipped"]
                : ""
              : filled
                ? "bg-gold"
                : "";
          const live = side === "you" && !filled && i === current;
          return (
            <span
              key={i}
              className={`relative flex-1 overflow-hidden rounded-full bg-border ${
                live ? "ring-2 ring-accent/60" : ""
              }`}
            >
              <span
                className={`absolute inset-0 origin-left rounded-full transition-transform duration-500 ease-out motion-reduce:transition-none ${
                  filled ? `scale-x-100 ${cell}` : "scale-x-0"
                }`}
              />
            </span>
          );
        })}
      </div>
    </div>
  );
}

/** Both lanes, you on top. */
export function RaceBoard({
  you,
  opponent,
  total,
  marks,
  current,
  opponentAnswered,
  opponentFinished,
  opponentOffline,
  youFinished = false,
}: {
  you: { name: string };
  opponent: { name: string; bot?: boolean };
  total: number;
  marks: RoundMark[];
  current?: number;
  opponentAnswered: number;
  opponentFinished: boolean;
  opponentOffline: boolean;
  youFinished?: boolean;
}) {
  const t = useTranslations("Arena");
  return (
    <div className="surface-raised-sm flex flex-none flex-col gap-2 rounded-xl border border-border bg-card px-3 py-2">
      <RaceLane
        side="you"
        name={t("you")}
        total={total}
        marks={marks}
        current={current}
        finished={youFinished}
        label={t("progressYou", { done: marks.length, total })}
      />
      <RaceLane
        side="opponent"
        name={opponent.bot ? t("botName") : opponent.name}
        bot={opponent.bot}
        total={total}
        answered={opponentAnswered}
        finished={opponentFinished}
        offline={opponentOffline}
        label={t("progressOpponent", { done: opponentAnswered, total })}
      />
    </div>
  );
}

/** Per-question comparison on the result screen: both rows of verdicts. */
export function MarksGrid({
  total,
  you,
  opponent,
  opponentName,
}: {
  total: number;
  you: RoundMark[];
  opponent: RoundMark[];
  opponentName: string;
}) {
  const t = useTranslations("Arena");
  const row = (marks: RoundMark[], label: string) => (
    <div className="flex items-center gap-2">
      <span className="w-16 flex-none truncate text-xs font-bold text-muted-foreground">{label}</span>
      <div className="flex flex-1 gap-1">
        {Array.from({ length: total }, (_, i) => {
          const m = marks[i];
          return (
            <span
              key={i}
              title={`${i + 1}`}
              className={`h-3 flex-1 rounded-sm ${m ? MARK_CELL[m] : "bg-border"}`}
            />
          );
        })}
      </div>
    </div>
  );
  return (
    <div className="flex w-full flex-col gap-1.5" aria-label={t("roundsTitle")}>
      {row(you, t("you"))}
      {row(opponent, opponentName)}
    </div>
  );
}
