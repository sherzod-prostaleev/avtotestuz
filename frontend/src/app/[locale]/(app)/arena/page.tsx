"use client";

import { useCallback, useEffect, useReducer, useRef, useState } from "react";
import Link from "next/link";
import { useLocale, useTranslations } from "next-intl";
import { ArrowLeft, Bot, Crown, Flag, RefreshCw, Swords, X } from "lucide-react";
import { apiGet } from "@/lib/api-client";
import { ArenaSocket } from "@/lib/arena-client";
import { normalizeInviteCode, sanitizeInviteInput } from "@/lib/arena-protocol";
import {
  arenaReducer,
  initialArenaState,
  isInMatch,
  type ArenaState,
  type Notice,
} from "@/lib/arena-state";
import { useUserStats } from "@/hooks/use-user-stats";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { ArenaLobby, type ArenaHistoryItem } from "@/components/arena/arena-lobby";
import {
  ArenaCountdown,
  ArenaQuestion,
  ArenaSearching,
  ArenaWaiting,
} from "@/components/arena/arena-duel";
import { ArenaResult } from "@/components/arena/arena-result";

type RatingDTO = { rating: number; medal: string };

/** Close codes the server uses (backend/internal/arena/protocol.go). */
const CLOSE_REPLACED = 4001;
/** Reconnect attempts before giving up and showing a retry button. The
 *  server holds a mid-duel seat for 20 s; 1+2+4+8+8 s covers it. */
const MAX_RETRIES = 5;

function retryDelayMs(attempt: number): number {
  return Math.min(8000, 1000 * 2 ** attempt);
}

function readInviteParam(): string {
  if (typeof window === "undefined") return "";
  const raw = new URLSearchParams(window.location.search).get("code") ?? "";
  return normalizeInviteCode(raw);
}

function dropInviteParam() {
  const url = new URL(window.location.href);
  if (!url.searchParams.has("code")) return;
  url.searchParams.delete("code");
  window.history.replaceState(window.history.state, "", url.toString());
}

export default function ArenaPage() {
  const t = useTranslations("Arena");
  const locale = useLocale();
  const { entitlement, loading: statsLoading } = useUserStats();
  const isVip = entitlement?.is_vip ?? false;

  const [state, dispatch] = useReducer(arenaReducer, initialArenaState);
  const stateRef = useRef<ArenaState>(state);
  useEffect(() => {
    stateRef.current = state;
  }, [state]);

  const [arenaEnabled, setArenaEnabled] = useState(true);
  const [rating, setRating] = useState<RatingDTO | null>(null);
  const [history, setHistory] = useState<ArenaHistoryItem[]>([]);
  const [joinCode, setJoinCode] = useState("");
  const [confirmForfeit, setConfirmForfeit] = useState(false);
  const [now, setNow] = useState(() => Date.now());

  const socketRef = useRef<ArenaSocket | null>(null);
  const retryRef = useRef<{ attempts: number; timer: number | null }>({ attempts: 0, timer: null });
  const pendingInviteRef = useRef<string>("");
  const connectRef = useRef<() => Promise<boolean>>(async () => false);

  useEffect(() => {
    void apiGet<{ arena_enabled: boolean }>("flags")
      .then((f) => setArenaEnabled(f.arena_enabled !== false))
      .catch(() => setArenaEnabled(true));
  }, []);

  const refreshMeta = useCallback(async () => {
    try {
      const [r, h] = await Promise.all([
        apiGet<RatingDTO>("me/arena/rating"),
        apiGet<ArenaHistoryItem[]>("me/arena/matches"),
      ]);
      setRating(r);
      setHistory(h.slice(0, 5));
    } catch {
      /* offline: the lobby shows its skeleton chip */
    }
  }, []);

  useEffect(() => {
    void refreshMeta();
  }, [refreshMeta]);

  // A finished duel changes the rating and the history.
  useEffect(() => {
    if (state.phase === "result") void refreshMeta();
  }, [state.phase, refreshMeta]);

  const scheduleRetry = useCallback((): boolean => {
    const r = retryRef.current;
    if (r.timer !== null) return true; // one pending retry at a time
    if (r.attempts >= MAX_RETRIES) return false;
    const delay = retryDelayMs(r.attempts);
    r.attempts += 1;
    r.timer = window.setTimeout(() => {
      r.timer = null;
      void connectRef.current();
    }, delay);
    return true;
  }, []);

  const connect = useCallback(async (): Promise<boolean> => {
    if (!socketRef.current) socketRef.current = new ArenaSocket();
    const socket = socketRef.current;
    if (socket.isOpen()) return true;
    dispatch({ type: "connecting" });
    try {
      await socket.connect({
        onMessage: (env) => dispatch({ type: "server", env, at: Date.now() }),
        onClose: (code) => {
          const replaced = code === CLOSE_REPLACED;
          const willRetry = !replaced && scheduleRetry();
          dispatch({ type: "socketClosed", replaced, willRetry });
        },
      });
      retryRef.current.attempts = 0;
      return true;
    } catch (err) {
      if (err instanceof Error && err.message === "arena_ws_superseded") return false;
      // Ticket mint or handshake failed; onClose may already have scheduled.
      if (!scheduleRetry()) dispatch({ type: "connectFailed" });
      return false;
    }
  }, [scheduleRetry]);

  useEffect(() => {
    connectRef.current = connect;
  }, [connect]);

  // Connect as soon as the lobby can be used: the open socket is what shows
  // who else is here, and it is how a reload lands back in a running duel.
  const canPlay = !statsLoading && isVip && arenaEnabled;
  useEffect(() => {
    if (!canPlay) return;
    const code = readInviteParam();
    if (code) {
      pendingInviteRef.current = code;
      setJoinCode(code);
    }
    void connect();
    const retry = retryRef.current;
    return () => {
      if (retry.timer !== null) window.clearTimeout(retry.timer);
      retry.timer = null;
      socketRef.current?.close();
      socketRef.current = null;
    };
  }, [canPlay, connect]);

  // An invite link (?code=) joins by itself once the lobby is ready.
  useEffect(() => {
    if (state.phase !== "lobby" || !pendingInviteRef.current) return;
    const code = pendingInviteRef.current;
    pendingInviteRef.current = "";
    dropInviteParam();
    socketRef.current?.send("invite.join", { code, locale });
  }, [state.phase, locale]);

  const ticking =
    state.phase === "countdown" || state.phase === "question" || state.phase === "searching";
  useEffect(() => {
    if (!ticking) return;
    setNow(Date.now());
    const id = window.setInterval(() => setNow(Date.now()), 250);
    return () => window.clearInterval(id);
  }, [ticking]);

  const send = useCallback(
    async (type: string, data: Record<string, unknown> = {}): Promise<boolean> => {
      if (!(await connect())) return false;
      const ok = socketRef.current?.send(type, data) ?? false;
      if (!ok) dispatch({ type: "connectFailed" });
      return ok;
    },
    [connect]
  );

  const findMatch = useCallback(() => void send("queue.join", { locale }), [send, locale]);
  const practiceBot = useCallback(() => void send("bot.start", { locale }), [send, locale]);
  const createInvite = useCallback(() => void send("invite.create", { locale }), [send, locale]);
  const cancelInvite = useCallback(() => void send("invite.cancel"), [send]);
  const joinInvite = useCallback(() => {
    const code = normalizeInviteCode(joinCode);
    if (code) void send("invite.join", { code, locale });
  }, [send, joinCode, locale]);
  const cancelSearch = useCallback(() => {
    socketRef.current?.send("queue.leave");
    dispatch({ type: "searchCancelled" });
  }, []);

  const answer = useCallback((answerId: string) => {
    const s = stateRef.current;
    const m = s.match;
    if (s.phase !== "question" || !m || m.pending) return;
    dispatch({ type: "selectAnswer", answerId });
    const ok =
      socketRef.current?.send("answer", { match_id: m.id, index: m.index, answer_id: answerId }) ?? false;
    if (!ok) dispatch({ type: "answerNotSent" });
  }, []);

  const inviteUrl = (code: string) =>
    `${window.location.origin}/${locale}/arena?code=${encodeURIComponent(code)}`;

  const shareInvite = (code: string) => {
    void navigator
      .share({ title: t("title"), text: t("shareText", { code }), url: inviteUrl(code) })
      .catch(() => {
        /* the user closed the share sheet */
      });
  };

  const copyInvite = async (code: string): Promise<boolean> => {
    try {
      await navigator.clipboard.writeText(`${t("shareText", { code })}\n${inviteUrl(code)}`);
      return true;
    } catch {
      return false;
    }
  };

  const playAgain = () => {
    const mode = state.result?.mode ?? state.lastMode;
    dispatch({ type: "backToLobby" });
    if (mode === "bot") practiceBot();
    else if (mode === "friend") createInvite();
    else findMatch();
  };

  if (statsLoading) {
    return (
      <div className="mx-auto max-w-2xl px-4 py-10">
        <p className="text-sm text-muted-foreground">{t("loading")}</p>
      </div>
    );
  }

  if (!isVip || !arenaEnabled) {
    return (
      <div className="mx-auto max-w-xl px-4 py-10">
        <Link
          href={`/${locale}/dashboard`}
          className="mb-6 inline-flex min-h-11 items-center gap-2 text-sm font-bold text-muted-foreground hover:text-foreground"
        >
          <ArrowLeft className="h-4 w-4" /> {t("back")}
        </Link>
        <div className="rounded-2xl border border-border bg-card p-8 text-center">
          {isVip ? (
            <Swords className="mx-auto mb-4 h-10 w-10 text-muted-foreground" aria-hidden />
          ) : (
            <Crown className="mx-auto mb-4 h-10 w-10 text-gold" aria-hidden />
          )}
          <h1 className="font-display text-2xl font-extrabold text-foreground">{t("title")}</h1>
          <p className="mt-3 text-sm text-muted-foreground">
            {isVip ? t("disabledBody") : t("vipLockBody")}
          </p>
          {!isVip && (
            <Link
              href={`/${locale}/premium`}
              className="mt-6 inline-flex min-h-12 items-center justify-center rounded-2xl border-b-4 border-gold-shadow bg-gold px-7 text-base font-extrabold text-slate-950 shadow-3d-gold"
            >
              {t("goPremium")}
            </Link>
          )}
        </div>
      </div>
    );
  }

  const { phase, match, result } = state;
  const inMatch = isInMatch(phase);
  const lobbyLike = phase === "idle" || phase === "connecting" || phase === "lobby";
  // Every in-duel screen is exactly one phone screen; the lobby scrolls.
  const fit = lobbyLike ? "" : "mobile-fit-screen min-h-0 md:min-h-[620px]";
  const secondsLeft = match?.deadline ? Math.max(0, Math.ceil((match.deadline - now) / 1000)) : 0;
  const countdownLeft = match ? Math.ceil((match.startsAt - now) / 1000) : 0;
  const searchElapsed = state.searchStartedAt
    ? Math.max(0, Math.floor((now - state.searchStartedAt) / 1000))
    : 0;

  return (
    <div
      data-testid="arena"
      className={`mx-auto flex w-full max-w-2xl flex-col gap-3 px-3 pt-3 md:px-4 md:pt-8 ${fit}`}
    >
      {state.notice && (
        <NoticeBar
          notice={state.notice}
          onDismiss={() => dispatch({ type: "dismissNotice" })}
          onPracticeBot={practiceBot}
        />
      )}

      {lobbyLike && (
        <ArenaLobby
          busy={phase !== "lobby"}
          online={state.online}
          rating={rating}
          history={history}
          invite={state.invite}
          joinCode={joinCode}
          onJoinCodeChange={(v) => setJoinCode(sanitizeInviteInput(v))}
          onFindMatch={findMatch}
          onPracticeBot={practiceBot}
          onCreateInvite={createInvite}
          onCancelInvite={cancelInvite}
          onJoinInvite={joinInvite}
          onShareInvite={shareInvite}
          onCopyInvite={copyInvite}
        />
      )}

      {phase === "error" && (
        <div className="flex flex-1 flex-col items-center justify-center gap-4 py-10 text-center">
          <Swords aria-hidden="true" className="h-10 w-10 text-muted-foreground" />
          <button
            type="button"
            onClick={() => {
              retryRef.current.attempts = 0;
              void connect();
            }}
            className="btn-3d-primary inline-flex min-h-12 items-center gap-2 rounded-xl px-6 font-display text-base font-extrabold"
          >
            <RefreshCw aria-hidden="true" className="h-4 w-4" />
            {state.notice?.key === "replaced" ? t("continueHere") : t("reconnect")}
          </button>
          <Link
            href={`/${locale}/dashboard`}
            className="inline-flex min-h-11 items-center gap-2 text-sm font-bold text-muted-foreground"
          >
            <ArrowLeft className="h-4 w-4" /> {t("back")}
          </Link>
        </div>
      )}

      {phase === "searching" && (
        <ArenaSearching
          elapsedSec={searchElapsed}
          online={state.online}
          onCancel={cancelSearch}
          onPracticeBot={() => {
            cancelSearch();
            practiceBot();
          }}
        />
      )}

      {phase === "countdown" && match && <ArenaCountdown match={match} secondsLeft={countdownLeft} />}

      {(phase === "question" || phase === "reveal") && match && (
        <ArenaQuestion
          match={match}
          phase={phase}
          secondsLeft={secondsLeft}
          onAnswer={answer}
          onForfeit={() => setConfirmForfeit(true)}
        />
      )}

      {phase === "waiting" && match && (
        <ArenaWaiting match={match} onForfeit={() => setConfirmForfeit(true)} />
      )}

      {phase === "result" && result && (
        <ArenaResult
          result={result}
          onPlayAgain={playAgain}
          onBackToLobby={() => dispatch({ type: "backToLobby" })}
        />
      )}

      <ConfirmDialog
        open={confirmForfeit && inMatch}
        title={t("forfeitTitle")}
        description={t("forfeitBody")}
        confirmLabel={t("forfeitConfirm")}
        cancelLabel={t("forfeitCancel")}
        icon={<Flag className="h-5 w-5 text-danger" aria-hidden />}
        onCancel={() => setConfirmForfeit(false)}
        onConfirm={() => {
          setConfirmForfeit(false);
          socketRef.current?.send("match.leave");
        }}
      />
    </div>
  );
}

function NoticeBar({
  notice,
  onDismiss,
  onPracticeBot,
}: {
  notice: Notice;
  onDismiss: () => void;
  onPracticeBot: () => void;
}) {
  const t = useTranslations("Arena");
  const text = {
    timeout: t("noticeTimeout"),
    connectError: t("noticeConnectError"),
    reconnecting: t("noticeReconnecting"),
    replaced: t("noticeReplaced"),
    vipRequired: t("noticeVipRequired"),
    alreadyInMatch: t("noticeAlreadyInMatch"),
    rateLimited: t("noticeRateLimited"),
    inviteInvalid: t("noticeInviteInvalid"),
    inviteSelf: t("noticeInviteSelf"),
    inviteHostAway: t("noticeInviteHostAway"),
    notEnoughQuestions: t("noticeNotEnoughQuestions"),
    serverBusy: t("noticeServerBusy"),
    opponentDisconnected: t("noticeOpponentDisconnected"),
    opponentBack: t("noticeOpponentBack"),
    matchEndedOffline: t("noticeMatchEndedOffline"),
  }[notice.key];
  const tone =
    notice.tone === "error"
      ? "border-danger/40 bg-danger/10"
      : "border-accent/40 bg-accent/10";
  return (
    <div
      role={notice.tone === "error" ? "alert" : "status"}
      className={`flex flex-none items-start gap-2 rounded-xl border px-3 py-2 text-sm ${tone}`}
    >
      <p className="min-w-0 flex-1 py-1">{text}</p>
      {notice.key === "timeout" && (
        <button
          type="button"
          onClick={onPracticeBot}
          className="inline-flex min-h-9 flex-none items-center gap-1.5 rounded-lg border border-border bg-card px-2.5 text-xs font-bold"
        >
          <Bot aria-hidden="true" className="h-4 w-4 text-accent" />
          {t("practiceBot")}
        </button>
      )}
      {notice.key !== "reconnecting" && (
        <button
          type="button"
          onClick={onDismiss}
          aria-label={t("dismiss")}
          className="-mr-1 flex h-9 w-9 flex-none items-center justify-center rounded-lg text-muted-foreground hover:text-foreground"
        >
          <X aria-hidden="true" className="h-4 w-4" />
        </button>
      )}
    </div>
  );
}
