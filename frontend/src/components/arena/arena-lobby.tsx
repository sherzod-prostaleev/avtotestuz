"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import {
  Bot,
  Check,
  ChevronRight,
  Copy,
  Share2,
  Swords,
  Trophy,
  Users,
  X,
} from "lucide-react";
import { INVITE_CODE_LEN, sanitizeInviteInput } from "@/lib/arena-protocol";
import { useMedalName, signed } from "@/components/arena/arena-parts";

export type ArenaHistoryItem = {
  match_id: string;
  mode?: string;
  outcome: string | null;
  score: number;
  correct_count?: number;
  rating_delta: number | null;
  opponent_name?: string | null;
  finished_at: string | null;
};

export interface ArenaLobbyProps {
  busy: boolean;
  online: number | null;
  rating: { rating: number; medal: string } | null;
  history: ArenaHistoryItem[];
  invite: { code: string } | null;
  joinCode: string;
  onJoinCodeChange: (value: string) => void;
  onFindMatch: () => void;
  onPracticeBot: () => void;
  onCreateInvite: () => void;
  onCancelInvite: () => void;
  onJoinInvite: () => void;
  onShareInvite: (code: string) => void;
  onCopyInvite: (code: string) => Promise<boolean>;
}

export function ArenaLobby({
  busy,
  online,
  rating,
  history,
  invite,
  joinCode,
  onJoinCodeChange,
  onFindMatch,
  onPracticeBot,
  onCreateInvite,
  onCancelInvite,
  onJoinInvite,
  onShareInvite,
  onCopyInvite,
}: ArenaLobbyProps) {
  const t = useTranslations("Arena");
  const medalName = useMedalName();
  const [copied, setCopied] = useState(false);
  const canShare = typeof navigator !== "undefined" && typeof navigator.share === "function";

  function outcomeName(outcome: string | null): string {
    if (outcome === "won") return t("outcomeWon");
    if (outcome === "lost") return t("outcomeLost");
    return t("draw");
  }

  return (
    <div className="flex flex-col gap-3 pb-4 md:gap-4">
      <header className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <h1 className="flex items-center gap-2 font-display text-2xl font-extrabold leading-tight tracking-tight md:text-3xl">
            <Swords aria-hidden="true" className="h-6 w-6 flex-none text-accent md:h-7 md:w-7" />
            {t("title")}
          </h1>
          <p className="text-sm text-muted-foreground">{t("subtitleShort")}</p>
        </div>
        {rating ? (
          <div className="flex h-11 flex-none items-center gap-[7px] rounded-xl border border-border bg-card px-3">
            <Trophy aria-hidden="true" className="h-[17px] w-[17px] text-gold" />
            <span className="font-display text-base font-extrabold tabular-nums">{rating.rating}</span>
            <span className="text-xs text-muted-foreground">{medalName(rating.medal)}</span>
          </div>
        ) : (
          <span aria-hidden="true" className="h-11 w-[122px] flex-none animate-pulse rounded-xl bg-card" />
        )}
      </header>

      <section className="surface-raised flex flex-col gap-3 rounded-2xl border border-border bg-card p-4">
        <div className="flex items-center justify-between gap-2">
          <p className="text-sm font-bold">{t("findMatchHint")}</p>
          {online !== null && (
            <span className="inline-flex flex-none items-center gap-1.5 text-xs font-bold text-muted-foreground">
              <span aria-hidden="true" className="relative flex h-2 w-2">
                <span className="absolute inset-0 rounded-full bg-success motion-safe:animate-ping" />
                <span className="relative h-2 w-2 rounded-full bg-success" />
              </span>
              {t("onlineNow", { count: online })}
            </span>
          )}
        </div>
        <button
          type="button"
          onClick={onFindMatch}
          disabled={busy}
          className="btn-3d-primary flex min-h-[52px] w-full items-center justify-center gap-2 rounded-xl font-display text-lg font-extrabold disabled:opacity-60"
        >
          <Users aria-hidden="true" className="h-5 w-5" strokeWidth={2.5} />
          {t("findMatch")}
        </button>
        <button
          type="button"
          onClick={onPracticeBot}
          disabled={busy}
          className="flex min-h-12 w-full items-center justify-center gap-2 rounded-xl border border-border bg-background text-base font-bold disabled:opacity-60"
        >
          <Bot aria-hidden="true" className="h-5 w-5 text-accent" />
          {t("practiceBot")}
          <span className="text-xs font-semibold text-muted-foreground">· {t("unratedShort")}</span>
        </button>
      </section>

      <section className="surface-raised flex flex-col gap-3 rounded-2xl border border-border bg-card p-4">
        <div>
          <h2 className="text-base font-bold">{t("inviteTitle")}</h2>
          <p className="mt-0.5 text-xs text-muted-foreground">{t("inviteHint")}</p>
        </div>

        {invite ? (
          <div className="flex flex-col gap-3 rounded-xl border border-accent/40 bg-accent/10 p-3">
            <div className="flex items-center justify-between gap-2">
              <span className="text-xs font-bold uppercase tracking-[0.08em] text-muted-foreground">
                {t("inviteYourCode")}
              </span>
              <button
                type="button"
                onClick={onCancelInvite}
                aria-label={t("cancelInvite")}
                className="-mr-2 flex h-9 w-9 items-center justify-center rounded-lg text-muted-foreground hover:text-foreground"
              >
                <X aria-hidden="true" className="h-4 w-4" />
              </button>
            </div>
            <p
              className="text-center font-mono text-3xl font-extrabold tracking-[0.3em] text-accent"
              aria-live="polite"
            >
              {invite.code}
            </p>
            <div className="flex gap-2">
              <button
                type="button"
                onClick={async () => {
                  const ok = await onCopyInvite(invite.code);
                  setCopied(ok);
                  if (ok) window.setTimeout(() => setCopied(false), 2000);
                }}
                className="flex min-h-11 flex-1 items-center justify-center gap-2 rounded-xl border border-border bg-card text-sm font-bold"
              >
                {copied ? (
                  <Check aria-hidden="true" className="h-4 w-4 text-success" />
                ) : (
                  <Copy aria-hidden="true" className="h-4 w-4" />
                )}
                {copied ? t("copied") : t("copyCode")}
              </button>
              {canShare && (
                <button
                  type="button"
                  onClick={() => onShareInvite(invite.code)}
                  className="btn-3d-primary flex min-h-11 flex-1 items-center justify-center gap-2 rounded-xl text-sm font-extrabold"
                >
                  <Share2 aria-hidden="true" className="h-4 w-4" />
                  {t("shareInvite")}
                </button>
              )}
            </div>
            <p className="flex items-center justify-center gap-2 text-xs text-muted-foreground">
              <span
                aria-hidden="true"
                className="h-3 w-3 flex-none rounded-full border-2 border-accent border-t-transparent motion-safe:animate-spin"
              />
              {t("inviteWaiting")}
            </p>
          </div>
        ) : (
          <button
            type="button"
            onClick={onCreateInvite}
            disabled={busy}
            className="min-h-11 w-full rounded-xl border border-border text-sm font-bold disabled:opacity-60"
          >
            {t("createInvite")}
          </button>
        )}

        <form
          className="flex gap-2"
          onSubmit={(e) => {
            e.preventDefault();
            onJoinInvite();
          }}
        >
          <label className="sr-only" htmlFor="arena-join-code">
            {t("joinInviteLabel")}
          </label>
          <input
            id="arena-join-code"
            value={joinCode}
            onChange={(e) => onJoinCodeChange(sanitizeInviteInput(e.target.value))}
            placeholder={t("invitePlaceholder")}
            autoComplete="off"
            autoCapitalize="characters"
            spellCheck={false}
            inputMode="text"
            maxLength={INVITE_CODE_LEN}
            className="min-h-11 min-w-0 flex-1 rounded-xl border border-border bg-background px-3 text-center font-mono text-base font-bold uppercase tracking-[0.25em] text-foreground"
          />
          <button
            type="submit"
            disabled={busy || joinCode.length !== INVITE_CODE_LEN}
            className="btn-3d-primary flex min-h-11 flex-none items-center justify-center gap-1 rounded-xl px-4 text-sm font-extrabold disabled:opacity-50"
          >
            {t("joinInvite")}
            <ChevronRight aria-hidden="true" className="h-4 w-4" strokeWidth={2.5} />
          </button>
        </form>
      </section>

      <section className="rounded-2xl border border-border bg-card/60 p-4">
        <h2 className="mb-2 text-xs font-extrabold uppercase tracking-[0.08em] text-muted-foreground">
          {t("rulesTitle")}
        </h2>
        <ul className="flex flex-col gap-1.5 text-sm">
          {(["rule1", "rule2", "rule3", "rule4"] as const).map((key) => (
            <li key={key} className="flex gap-2">
              <span aria-hidden="true" className="mt-[9px] h-1.5 w-1.5 flex-none rounded-full bg-accent" />
              <span>{t(key)}</span>
            </li>
          ))}
        </ul>
      </section>

      {history.length > 0 && (
        <section>
          <h2 className="mb-1.5 text-xs font-extrabold uppercase tracking-[0.08em] text-muted-foreground">
            {t("historyTitle")}
          </h2>
          <ul className="overflow-hidden rounded-xl border border-border bg-card">
            {history.map((item, index) => {
              const won = item.outcome === "won";
              const lost = item.outcome === "lost";
              const dot = won ? "bg-success" : lost ? "bg-danger" : "bg-muted-foreground";
              const tone = won ? "text-success" : lost ? "text-danger" : "text-muted-foreground";
              const who =
                item.mode === "bot"
                  ? t("historyVsBot")
                  : item.opponent_name ?? t("opponent");
              const rated = item.mode !== "bot" && item.mode !== "friend";
              return (
                <li key={item.match_id}>
                  {index > 0 && <div aria-hidden="true" className="ml-[34px] h-px bg-border" />}
                  <div className="flex min-h-12 items-center gap-3 px-3.5 py-1.5">
                    <span aria-hidden="true" className={`h-2 w-2 flex-none rounded-full ${dot}`} />
                    <span className="min-w-0 flex-1">
                      <span className="block truncate text-sm font-semibold">
                        {outcomeName(item.outcome)} · {who}
                      </span>
                      {item.mode === "friend" && (
                        <span className="block text-xs text-muted-foreground">{t("historyFriend")}</span>
                      )}
                    </span>
                    <span className="text-sm font-bold tabular-nums text-muted-foreground">
                      {t("historyScore", { score: item.score })}
                    </span>
                    <span className={`w-[42px] flex-none text-right text-xs font-bold tabular-nums ${tone}`}>
                      {rated ? signed(item.rating_delta ?? 0) : "—"}
                    </span>
                  </div>
                </li>
              );
            })}
          </ul>
        </section>
      )}
    </div>
  );
}
