"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import Link from "next/link";
import { useLocale, useTranslations } from "next-intl";
import { Swords, Trophy, Users, Clock, Crown, ArrowLeft } from "lucide-react";
import { apiGet } from "@/lib/api-client";
import { ArenaSocket } from "@/lib/arena-client";
import {
  medalLabel,
  type ArenaEnvelope,
  type ArenaPhase,
  type QuestionPayload,
} from "@/lib/arena-protocol";
import { Button } from "@/components/ui/button";
import { useUserStats } from "@/hooks/use-user-stats";

type RatingDTO = { rating: number; medal: string };
type HistoryItem = {
  match_id: string;
  outcome: string | null;
  score: number;
  rating_delta: number | null;
  medal: string;
  finished_at: string | null;
};

type ScorePair = { you: number; opponent: number };

export default function ArenaPage() {
  const t = useTranslations("Arena");
  const locale = useLocale();
  const { entitlement, loading: statsLoading } = useUserStats();
  const isVip = entitlement?.is_vip ?? false;

  const socketRef = useRef<ArenaSocket | null>(null);
  const [phase, setPhase] = useState<ArenaPhase>("idle");
  const [statusMsg, setStatusMsg] = useState("");
  const [opponentName, setOpponentName] = useState("");
  const [matchId, setMatchId] = useState<string | null>(null);
  const [question, setQuestion] = useState<QuestionPayload | null>(null);
  const [qIndex, setQIndex] = useState(0);
  const [qTotal, setQTotal] = useState(10);
  const [deadlineMs, setDeadlineMs] = useState(0);
  const [nowMs, setNowMs] = useState(() => Date.now());
  const [selected, setSelected] = useState<string | null>(null);
  const [acked, setAcked] = useState(false);
  const [revealCorrect, setRevealCorrect] = useState<string | null>(null);
  const [score, setScore] = useState<ScorePair>({ you: 0, opponent: 0 });
  const [result, setResult] = useState<{
    outcome: string;
    reason: string;
    score: ScorePair;
    correct: ScorePair;
    ratingDelta?: number;
    medal?: string;
  } | null>(null);
  const [inviteCode, setInviteCode] = useState("");
  const [joinCode, setJoinCode] = useState("");
  const [rating, setRating] = useState<RatingDTO | null>(null);
  const [history, setHistory] = useState<HistoryItem[]>([]);
  const [remaining, setRemaining] = useState(15);

  const refreshMeta = useCallback(async () => {
    try {
      const [r, h] = await Promise.all([
        apiGet<RatingDTO>("me/arena/rating"),
        apiGet<HistoryItem[]>("me/arena/matches"),
      ]);
      setRating(r);
      setHistory(h.slice(0, 8));
    } catch {
      /* ignore when offline */
    }
  }, []);

  const handleEnv = useCallback(
    (env: ArenaEnvelope) => {
      const d = (env.d ?? {}) as Record<string, unknown>;
      switch (env.t) {
        case "hello":
          setPhase("lobby");
          setStatusMsg("");
          break;
        case "queue.joined":
          setPhase("searching");
          setStatusMsg(t("searching"));
          break;
        case "queue.timeout":
          setPhase("lobby");
          setStatusMsg(t("timeout"));
          break;
        case "match.found": {
          setMatchId(String(d.match_id ?? ""));
          const opp = d.opponent as { name?: string } | undefined;
          setOpponentName(opp?.name ?? t("opponent"));
          setQTotal(Number(d.question_count ?? 10));
          setPhase("countdown");
          setStatusMsg(t("matchFound", { name: opp?.name ?? t("opponent") }));
          break;
        }
        case "question": {
          setPhase("question");
          setQIndex(Number(d.index ?? 0));
          setQTotal(Number(d.total ?? 10));
          setDeadlineMs(Number(d.deadline_ms ?? 0));
          setSelected(null);
          setAcked(false);
          setRevealCorrect(null);
          const q = d.question as QuestionPayload;
          setQuestion(q);
          break;
        }
        case "answer.ack":
          setAcked(true);
          break;
        case "question.result": {
          setPhase("reveal");
          setRevealCorrect(String(d.correct_answer_id ?? ""));
          const sc = d.score as ScorePair | undefined;
          if (sc) setScore(sc);
          break;
        }
        case "opponent.status":
          setStatusMsg(
            d.state === "disconnected" ? t("opponentDisconnected") : t("opponentConnected")
          );
          break;
        case "match.end": {
          setPhase("result");
          setResult({
            outcome: String(d.outcome ?? "draw"),
            reason: String(d.reason ?? ""),
            score: (d.score as ScorePair) ?? { you: 0, opponent: 0 },
            correct: (d.correct as ScorePair) ?? { you: 0, opponent: 0 },
            ratingDelta: typeof d.rating_delta === "number" ? d.rating_delta : undefined,
            medal: typeof d.medal === "string" ? d.medal : undefined,
          });
          void refreshMeta();
          break;
        }
        case "invite.created":
          setInviteCode(String(d.code ?? ""));
          break;
        case "error": {
          const code = String(d.code ?? "");
          if (code === "vip_required") {
            setPhase("error");
            setStatusMsg(t("vipRequired"));
          } else {
            setStatusMsg(String(d.message ?? code));
          }
          break;
        }
        default:
          break;
      }
    },
    [t, refreshMeta]
  );

  useEffect(() => {
    void refreshMeta();
  }, [refreshMeta]);

  useEffect(() => {
    const id = window.setInterval(() => setNowMs(Date.now()), 200);
    return () => window.clearInterval(id);
  }, []);

  useEffect(() => {
    if (!deadlineMs) {
      setRemaining(0);
      return;
    }
    const left = Math.max(0, Math.ceil((deadlineMs - nowMs) / 1000));
    setRemaining(left);
  }, [deadlineMs, nowMs]);

  useEffect(() => {
    return () => {
      socketRef.current?.close();
    };
  }, []);

  const ensureSocket = async () => {
    if (!socketRef.current) {
      socketRef.current = new ArenaSocket();
    }
    setPhase("connecting");
    await socketRef.current.connect({
      onMessage: handleEnv,
      onOpen: () => setPhase("lobby"),
      onError: () => {
        setPhase("error");
        setStatusMsg(t("connectError"));
      },
      onClose: () => {
        setPhase((p) => (p === "result" ? p : "idle"));
      },
    });
  };

  const findMatch = async () => {
    try {
      await ensureSocket();
      socketRef.current?.send("queue.join", { locale });
    } catch {
      setPhase("error");
      setStatusMsg(t("connectError"));
    }
  };

  const cancelSearch = () => {
    socketRef.current?.send("queue.leave");
    setPhase("lobby");
    setStatusMsg("");
  };

  const createInvite = async () => {
    try {
      await ensureSocket();
      socketRef.current?.send("invite.create");
    } catch {
      setStatusMsg(t("connectError"));
    }
  };

  const joinInvite = async () => {
    if (!joinCode.trim()) return;
    try {
      await ensureSocket();
      socketRef.current?.send("invite.join", { code: joinCode.trim(), locale });
    } catch {
      setStatusMsg(t("connectError"));
    }
  };

  const answer = (answerId: string) => {
    if (phase !== "question" || acked || !matchId) return;
    setSelected(answerId);
    socketRef.current?.send("answer", {
      match_id: matchId,
      index: qIndex,
      answer_id: answerId,
    });
  };

  const leaveMatch = () => {
    socketRef.current?.send("match.leave");
    setPhase("lobby");
  };

  const playAgain = () => {
    setResult(null);
    setQuestion(null);
    setMatchId(null);
    setPhase("lobby");
    void findMatch();
  };

  if (statsLoading) {
    return (
      <div className="mx-auto max-w-3xl px-4 py-10">
        <p className="text-sm text-muted-foreground">{t("loading")}</p>
      </div>
    );
  }

  if (!isVip) {
    return (
      <div className="mx-auto max-w-xl px-4 py-10">
        <Link
          href={`/${locale}/dashboard`}
          className="mb-6 inline-flex min-h-11 items-center gap-2 text-sm font-bold text-muted-foreground hover:text-foreground"
        >
          <ArrowLeft className="h-4 w-4" /> {t("back")}
        </Link>
        <div className="rounded-2xl border border-border bg-card p-8 text-center">
          <Crown className="mx-auto mb-4 h-10 w-10 text-gold" aria-hidden />
          <h1 className="font-display text-2xl font-extrabold text-foreground">{t("title")}</h1>
          <p className="mt-3 text-sm text-muted-foreground">{t("vipLockBody")}</p>
          <Link
            href={`/${locale}/premium`}
            className="mt-6 inline-flex min-h-12 items-center justify-center rounded-2xl border-b-4 border-gold-shadow bg-gold px-7 text-base font-extrabold text-slate-950 shadow-3d-gold"
          >
            {t("goPremium")}
          </Link>
        </div>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-3xl px-4 py-6 md:py-10">
      <div className="mb-6 flex flex-wrap items-center justify-between gap-3">
        <Link
          href={`/${locale}/dashboard`}
          className="inline-flex min-h-11 items-center gap-2 text-sm font-bold text-muted-foreground hover:text-foreground"
        >
          <ArrowLeft className="h-4 w-4" /> {t("back")}
        </Link>
        {rating && (
          <div className="flex items-center gap-2 rounded-xl border border-border bg-card px-3 py-2 text-sm">
            <Trophy className="h-4 w-4 text-accent" aria-hidden />
            <span className="font-extrabold text-foreground">{rating.rating}</span>
            <span className="text-muted-foreground">{medalLabel(rating.medal)}</span>
          </div>
        )}
      </div>

      <header className="mb-8">
        <div className="flex items-center gap-3">
          <Swords className="h-8 w-8 text-accent" aria-hidden />
          <h1 className="font-display text-3xl font-extrabold tracking-tight text-foreground">
            {t("title")}
          </h1>
        </div>
        <p className="mt-2 max-w-xl text-sm text-muted-foreground">{t("subtitle")}</p>
      </header>

      {statusMsg && phase !== "question" && phase !== "reveal" && (
        <p className="mb-4 rounded-xl border border-border bg-muted/40 px-4 py-3 text-sm text-foreground">
          {statusMsg}
        </p>
      )}

      {(phase === "idle" || phase === "lobby" || phase === "connecting" || phase === "error") && (
        <section className="space-y-4">
          <Button
            size="lg"
            className="w-full sm:w-auto"
            onClick={() => void findMatch()}
            disabled={phase === "connecting"}
          >
            <Users className="mr-2 h-4 w-4" /> {t("findMatch")}
          </Button>

          <div className="rounded-2xl border border-border bg-card p-5">
            <h2 className="text-sm font-extrabold text-foreground">{t("inviteTitle")}</h2>
            <p className="mt-1 text-xs text-muted-foreground">{t("inviteHint")}</p>
            <div className="mt-4 flex flex-wrap gap-2">
              <Button variant="outline" onClick={() => void createInvite()}>
                {t("createInvite")}
              </Button>
              {inviteCode && (
                <span className="inline-flex min-h-11 items-center rounded-xl border border-accent bg-accent/10 px-4 font-mono text-sm font-extrabold text-accent">
                  {inviteCode}
                </span>
              )}
            </div>
            <div className="mt-4 flex flex-wrap gap-2">
              <input
                value={joinCode}
                onChange={(e) => setJoinCode(e.target.value.toUpperCase())}
                placeholder={t("invitePlaceholder")}
                className="min-h-11 flex-1 rounded-xl border border-border bg-background px-3 text-sm font-bold uppercase tracking-widest text-foreground"
                maxLength={12}
              />
              <Button variant="outline" onClick={() => void joinInvite()}>
                {t("joinInvite")}
              </Button>
            </div>
          </div>
        </section>
      )}

      {phase === "searching" && (
        <section className="rounded-2xl border border-border bg-card p-8 text-center">
          <div className="mx-auto mb-4 h-10 w-10 animate-spin rounded-full border-2 border-accent border-t-transparent motion-reduce:animate-none" />
          <p className="font-display text-xl font-extrabold">{t("searching")}</p>
          <Button variant="ghost" className="mt-6" onClick={cancelSearch}>
            {t("cancelSearch")}
          </Button>
        </section>
      )}

      {phase === "countdown" && (
        <section className="rounded-2xl border border-accent/40 bg-card p-8 text-center">
          <p className="text-sm text-muted-foreground">{t("vs")}</p>
          <p className="mt-2 font-display text-2xl font-extrabold">{opponentName}</p>
          <p className="mt-4 text-sm font-bold text-accent">{t("starting")}</p>
        </section>
      )}

      {(phase === "question" || phase === "reveal") && question && (
        <section className="space-y-4">
          <div className="flex flex-wrap items-center justify-between gap-2 text-sm font-bold">
            <span>
              {t("questionProgress", { current: qIndex + 1, total: qTotal })}
            </span>
            <span className="inline-flex min-h-11 items-center gap-2 rounded-xl border border-border bg-card px-3">
              <Clock className="h-4 w-4 text-accent" aria-hidden />
              {remaining}s
            </span>
            <span className="font-display">
              {score.you} : {score.opponent}
            </span>
          </div>

          <div className="rounded-2xl border border-border bg-card p-5">
            <p className="text-base font-bold leading-relaxed text-foreground">{question.text}</p>
            {question.image_url && (
              // eslint-disable-next-line @next/next/no-img-element
              <img
                src={question.image_url}
                alt=""
                className="mt-4 max-h-56 w-full rounded-xl object-contain"
              />
            )}
          </div>

          <ul className="space-y-2">
            {question.answers.map((a) => {
              const isSel = selected === a.id;
              const isCorrect = revealCorrect === a.id;
              const showReveal = phase === "reveal";
              let cls =
                "w-full min-h-11 rounded-xl border px-4 py-3 text-left text-sm font-bold transition-colors";
              if (showReveal && isCorrect) {
                cls += " border-success bg-success/15 text-foreground";
              } else if (showReveal && isSel && !isCorrect) {
                cls += " border-danger bg-danger/15 text-foreground";
              } else if (isSel) {
                cls += " border-accent bg-accent/15 text-foreground";
              } else {
                cls += " border-border bg-card text-foreground hover:border-accent";
              }
              return (
                <li key={a.id}>
                  <button
                    type="button"
                    disabled={acked || phase !== "question"}
                    onClick={() => answer(a.id)}
                    className={cls}
                  >
                    {a.text}
                  </button>
                </li>
              );
            })}
          </ul>

          <Button variant="ghost" onClick={leaveMatch}>
            {t("forfeit")}
          </Button>
        </section>
      )}

      {phase === "result" && result && (
        <section className="rounded-2xl border border-border bg-card p-8 text-center">
          <h2 className="font-display text-3xl font-extrabold">
            {result.outcome === "won"
              ? t("youWon")
              : result.outcome === "lost"
                ? t("youLost")
                : t("draw")}
          </h2>
          <p className="mt-3 font-display text-2xl font-extrabold text-accent">
            {result.score.you} : {result.score.opponent}
          </p>
          <p className="mt-2 text-sm text-muted-foreground">
            {t("correctLine", {
              you: result.correct.you,
              opponent: result.correct.opponent,
            })}
          </p>
          {typeof result.ratingDelta === "number" && (
            <p className="mt-2 text-sm font-bold">
              {t("ratingDelta", { delta: result.ratingDelta > 0 ? `+${result.ratingDelta}` : String(result.ratingDelta) })}
              {result.medal ? ` · ${medalLabel(result.medal)}` : ""}
            </p>
          )}
          <div className="mt-6 flex flex-wrap justify-center gap-2">
            <Button size="lg" onClick={playAgain}>
              {t("playAgain")}
            </Button>
            <Button
              variant="outline"
              onClick={() => {
                setResult(null);
                setPhase("lobby");
              }}
            >
              {t("backToLobby")}
            </Button>
          </div>
        </section>
      )}

      {history.length > 0 && (phase === "idle" || phase === "lobby" || phase === "error") && (
        <section className="mt-10">
          <h2 className="mb-3 text-sm font-extrabold text-foreground">{t("historyTitle")}</h2>
          <ul className="space-y-2">
            {history.map((h) => (
              <li
                key={h.match_id}
                className="flex items-center justify-between rounded-xl border border-border bg-card px-4 py-3 text-sm"
              >
                <span className="font-bold capitalize">{h.outcome ?? "—"}</span>
                <span className="text-muted-foreground">{h.score} pts</span>
                <span className="font-bold text-accent">
                  {h.rating_delta != null && h.rating_delta > 0 ? "+" : ""}
                  {h.rating_delta ?? 0}
                </span>
              </li>
            ))}
          </ul>
        </section>
      )}
    </div>
  );
}
