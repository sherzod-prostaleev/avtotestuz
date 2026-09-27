"use client";

import { useTranslations } from "next-intl";
import { ArrowRight, Bot, Trophy } from "lucide-react";
import type { ResultView } from "@/lib/arena-state";
import { Avatar, MarksGrid, signed, useMedalName } from "@/components/arena/arena-parts";

export function ArenaResult({
  result,
  onPlayAgain,
  onBackToLobby,
}: {
  result: ResultView;
  onPlayAgain: () => void;
  onBackToLobby: () => void;
}) {
  const t = useTranslations("Arena");
  const medalName = useMedalName();
  const won = result.outcome === "won";
  const lost = result.outcome === "lost";
  const tone = won ? "text-success" : lost ? "text-danger" : "text-muted-foreground";
  const ring = won ? "bg-success/15 text-success" : lost ? "bg-danger/15 text-danger" : "bg-muted text-muted-foreground";
  const oppName = result.opponent.bot ? t("botName") : result.opponent.name || t("opponent");

  let reason: string | null = null;
  if (result.reason === "forfeit") reason = won ? t("reasonForfeitWon") : t("reasonForfeitLost");
  else if (result.reason === "both_disconnected") reason = t("reasonAbandoned");
  else if (result.reason === "server_shutdown") reason = t("reasonShutdown");

  const player = (name: string, side: "you" | "opponent", score: number, correct: number, bot?: boolean) => (
    <div className="flex min-w-0 flex-1 flex-col items-center gap-1.5 text-center">
      <Avatar name={name} side={side} bot={bot} />
      <p className="w-full truncate text-sm font-bold">{name}</p>
      <p className="font-display text-3xl font-extrabold leading-none tabular-nums">
        {score}
        <span className="ml-1 text-xs font-bold text-muted-foreground">{t("pointsLabel")}</span>
      </p>
      <p className="text-xs font-bold text-muted-foreground">
        {t("correctOf", { correct, total: result.total })}
      </p>
    </div>
  );

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-3">
      <div className="flex min-h-0 flex-1 flex-col items-center justify-center gap-3 overflow-y-auto text-center md:gap-4">
        <span aria-hidden="true" className={`hidden h-[76px] w-[76px] flex-none items-center justify-center rounded-full md:flex ${ring}`}>
          <Trophy className="h-9 w-9" />
        </span>
        <div role="status" aria-live="polite">
          <p className={`font-display text-3xl font-extrabold ${tone}`}>
            {won ? t("youWon") : lost ? t("youLost") : t("draw")}
          </p>
          {reason && <p className="mt-1 text-sm text-muted-foreground">{reason}</p>}
        </div>

        <div className="surface-raised-sm flex w-full max-w-md flex-none items-start gap-2 rounded-2xl border border-border bg-card px-3 py-3">
          {player(t("you"), "you", result.score.you, result.correct.you)}
          <span className="mt-10 font-display text-lg font-extrabold text-muted-foreground">:</span>
          {player(oppName, "opponent", result.score.opponent, result.correct.opponent, result.opponent.bot)}
        </div>

        <div className="w-full max-w-md flex-none rounded-xl border border-border bg-card px-3 py-2.5">
          <p className="mb-2 text-left text-xs font-extrabold uppercase tracking-[0.08em] text-muted-foreground">
            {t("roundsTitle")}
          </p>
          <MarksGrid
            total={result.total}
            you={result.marks.you}
            opponent={result.marks.opponent}
            opponentName={oppName}
          />
        </div>

        {result.rated ? (
          <div className="flex flex-wrap items-center justify-center gap-2.5 rounded-xl border border-border bg-card px-3.5 py-2.5">
            <span className="text-sm text-muted-foreground">{t("ratingLabel")}</span>
            <span className="font-display text-base font-bold tabular-nums text-muted-foreground">
              {result.ratingBefore}
            </span>
            <ArrowRight aria-hidden="true" className="h-4 w-4 text-muted-foreground" />
            <span className="font-display text-lg font-extrabold tabular-nums">{result.ratingAfter}</span>
            <span
              className={`text-sm font-extrabold tabular-nums ${
                result.ratingDelta >= 0 ? "text-success" : "text-danger"
              }`}
            >
              {signed(result.ratingDelta)}
            </span>
            <span className="text-xs text-muted-foreground">{medalName(result.medal)}</span>
          </div>
        ) : (
          <p className="text-xs text-muted-foreground">{t("unratedNote")}</p>
        )}
      </div>

      <div className="mb-3 flex flex-none gap-2">
        <button
          type="button"
          onClick={onPlayAgain}
          className="btn-3d-primary flex min-h-[50px] flex-[1.2] items-center justify-center gap-2 rounded-xl px-2 font-display text-base font-extrabold leading-tight md:text-lg"
        >
          {result.mode === "bot" && <Bot aria-hidden="true" className="h-5 w-5" />}
          {result.mode === "bot"
            ? t("playAgainBot")
            : result.mode === "friend"
              ? t("playAgainFriend")
              : t("playAgain")}
        </button>
        <button
          type="button"
          onClick={onBackToLobby}
          className="min-h-[50px] flex-1 rounded-xl border border-border px-2 text-sm font-bold leading-tight text-muted-foreground"
        >
          {t("backToLobbyFull")}
        </button>
      </div>
    </div>
  );
}
