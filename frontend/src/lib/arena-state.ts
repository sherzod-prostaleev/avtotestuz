/**
 * Arena client state machine. Pure: every server frame and every local
 * gesture is an action, and the page only renders what this returns. Times
 * are kept in the *client's* clock — the server's absolute deadlines are
 * shifted by the offset measured from each frame's `server_time_ms`, so a
 * phone whose clock is a few seconds off still shows the right countdown.
 */
import {
  isPlayableQuestion,
  type AnswerResult,
  type ArenaEnvelope,
  type ArenaMode,
  type ArenaPhase,
  type PlayerCard,
  type QuestionPayload,
  type RoundMark,
} from "@/lib/arena-protocol";

export type NoticeKey =
  | "timeout"
  | "connectError"
  | "reconnecting"
  | "replaced"
  | "vipRequired"
  | "alreadyInMatch"
  | "rateLimited"
  | "inviteInvalid"
  | "inviteSelf"
  | "inviteHostAway"
  | "notEnoughQuestions"
  | "serverBusy"
  | "opponentDisconnected"
  | "opponentBack"
  | "matchEndedOffline";

export type Notice = { tone: "info" | "error"; key: NoticeKey };

export type OpponentView = PlayerCard & {
  answered: number;
  finished: boolean;
  connected: boolean;
};

export type MatchView = {
  id: string;
  mode: ArenaMode;
  total: number;
  questionTimeMs: number;
  /** Local-clock instant the first question is dealt. */
  startsAt: number;
  you: PlayerCard;
  opponent: OpponentView;
  index: number;
  question: QuestionPayload | null;
  /** Local-clock deadline of the current question; 0 when none is live. */
  deadline: number;
  selected: string | null;
  /** An answer is on the wire and its verdict has not come back yet. */
  pending: boolean;
  verdict: AnswerResult | null;
  marks: RoundMark[];
  score: number;
};

export type ResultView = {
  mode: ArenaMode;
  outcome: "won" | "lost" | "draw";
  reason: string;
  score: { you: number; opponent: number };
  correct: { you: number; opponent: number };
  marks: { you: RoundMark[]; opponent: RoundMark[] };
  total: number;
  rated: boolean;
  ratingBefore: number;
  ratingAfter: number;
  ratingDelta: number;
  medal: string;
  you: PlayerCard;
  opponent: PlayerCard;
};

export type ArenaState = {
  phase: ArenaPhase;
  online: number | null;
  notice: Notice | null;
  /** Local instant the current search started, for its elapsed clock. */
  searchStartedAt: number;
  invite: { code: string; expiresAt: number } | null;
  match: MatchView | null;
  result: ResultView | null;
  /** The mode to repeat on "play again" after a result. */
  lastMode: ArenaMode | null;
};

export const initialArenaState: ArenaState = {
  phase: "idle",
  online: null,
  notice: null,
  searchStartedAt: 0,
  invite: null,
  match: null,
  result: null,
  lastMode: null,
};

export type ArenaAction =
  | { type: "server"; env: ArenaEnvelope; at: number }
  | { type: "connecting" }
  | { type: "socketClosed"; replaced: boolean; willRetry: boolean }
  | { type: "connectFailed" }
  | { type: "selectAnswer"; answerId: string }
  | { type: "answerNotSent" }
  | { type: "searchCancelled" }
  | { type: "backToLobby" }
  | { type: "dismissNotice" };

const IN_MATCH: ReadonlySet<ArenaPhase> = new Set([
  "countdown",
  "question",
  "reveal",
  "waiting",
]);

export function isInMatch(phase: ArenaPhase): boolean {
  return IN_MATCH.has(phase);
}

type Dict = Record<string, unknown>;

function num(v: unknown, fallback = 0): number {
  return typeof v === "number" && Number.isFinite(v) ? v : fallback;
}

function str(v: unknown, fallback = ""): string {
  return typeof v === "string" ? v : fallback;
}

function mode(v: unknown): ArenaMode {
  return v === "friend" || v === "bot" ? v : "ranked";
}

function card(v: unknown): PlayerCard {
  const d = (v ?? {}) as Dict;
  return {
    name: str(d.name),
    rating: num(d.rating, 1000),
    medal: str(d.medal, "bronze"),
    bot: d.bot === true,
  };
}

function marks(v: unknown): RoundMark[] {
  if (!Array.isArray(v)) return [];
  return v.map((m) => (m === "correct" || m === "wrong" ? m : "skipped"));
}

function markOf(r: AnswerResult): RoundMark {
  if (!r.answered) return "skipped";
  return r.correct ? "correct" : "wrong";
}

function asResult(v: unknown): AnswerResult | null {
  if (!v || typeof v !== "object") return null;
  const d = v as Dict;
  return {
    index: num(d.index),
    answered: d.answered === true,
    correct: d.correct === true,
    answer_id: typeof d.answer_id === "string" ? d.answer_id : undefined,
    correct_answer_id: str(d.correct_answer_id),
    points: num(d.points),
    score: num(d.score),
    response_ms: num(d.response_ms),
    next_in_ms: num(d.next_in_ms),
    last: d.last === true,
  };
}

/** server instant → local instant, using the frame's own clock reading. */
function toLocal(serverMs: number, d: Dict, at: number): number {
  if (!serverMs) return 0;
  const serverNow = num(d.server_time_ms);
  const offset = serverNow ? serverNow - at : 0;
  return serverMs - offset;
}

const ERROR_NOTICE: Record<string, NoticeKey> = {
  vip_required: "vipRequired",
  already_in_match: "alreadyInMatch",
  rate_limited: "rateLimited",
  invite_invalid: "inviteInvalid",
  invite_self: "inviteSelf",
  invite_host_away: "inviteHostAway",
  not_enough_questions: "notEnoughQuestions",
  server_busy: "serverBusy",
};

/** Answer-path rejections: the verdict will come from the clock instead. */
const ANSWER_ERRORS = new Set(["wrong_question", "too_late", "invalid_answer", "already_answered"]);

function onServer(state: ArenaState, env: ArenaEnvelope, at: number): ArenaState {
  const d = (env.d ?? {}) as Dict;
  switch (env.t) {
    case "hello": {
      const online = num(d.online, state.online ?? 0);
      // A host who reconnects gets their still-open code back.
      const inv = d.invite as Dict | undefined;
      const invite =
        inv && typeof inv.code === "string" && inv.code
          ? { code: inv.code, expiresAt: at + num(inv.expires_in_sec) * 1000 }
          : state.invite;
      if (d.in_match === true) {
        // A match.state resync follows; keep the screen until it lands.
        return { ...state, online, invite, notice: null };
      }
      if (isInMatch(state.phase)) {
        // We were mid-duel, and the server no longer has us in one: it ended
        // (forfeit after the reconnect grace) while this socket was down.
        return {
          ...state,
          online,
          phase: "lobby",
          match: null,
          notice: { tone: "error", key: "matchEndedOffline" },
        };
      }
      const keep = state.phase === "result" || state.phase === "searching";
      return {
        ...state,
        online,
        invite,
        phase: keep ? state.phase : "lobby",
        notice: state.notice?.key === "reconnecting" ? null : state.notice,
      };
    }
    case "queue.joined":
      return {
        ...state,
        phase: "searching",
        online: num(d.online, state.online ?? 0),
        searchStartedAt: at,
        notice: null,
        invite: null,
      };
    case "queue.timeout":
      if (state.phase !== "searching") return state;
      return {
        ...state,
        phase: "lobby",
        online: num(d.online, state.online ?? 0),
        notice: { tone: "info", key: "timeout" },
      };
    case "queue.left":
      return state.phase === "searching" ? { ...state, phase: "lobby" } : state;
    case "invite.created":
      return {
        ...state,
        invite: { code: str(d.code), expiresAt: at + num(d.expires_in_sec) * 1000 },
        notice: null,
      };
    case "invite.cancelled":
      return { ...state, invite: null };
    case "match.found": {
      const m = mode(d.mode);
      return {
        ...state,
        phase: "countdown",
        notice: null,
        invite: null,
        result: null,
        lastMode: m,
        match: {
          id: str(d.match_id),
          mode: m,
          total: num(d.question_count, 10),
          questionTimeMs: num(d.question_time_ms, 15000),
          startsAt: d.starts_at_ms
            ? toLocal(num(d.starts_at_ms), d, at)
            : at + num(d.starts_in_ms, 3000),
          you: card(d.you),
          opponent: { ...card(d.opponent), answered: 0, finished: false, connected: true },
          index: 0,
          question: null,
          deadline: 0,
          selected: null,
          pending: false,
          verdict: null,
          marks: [],
          score: 0,
        },
      };
    }
    case "question": {
      if (!state.match) return state;
      if (!isPlayableQuestion(d.question)) {
        return { ...state, notice: { tone: "error", key: "serverBusy" } };
      }
      return {
        ...state,
        phase: "question",
        match: {
          ...state.match,
          index: num(d.index),
          total: num(d.total, state.match.total),
          question: d.question,
          deadline: toLocal(num(d.deadline_ms), d, at),
          selected: null,
          pending: false,
          verdict: null,
        },
      };
    }
    case "answer.result": {
      const r = asResult(d);
      if (!state.match || !r) return state;
      const nextMarks = state.match.marks.slice(0, r.index);
      nextMarks[r.index] = markOf(r);
      return {
        ...state,
        phase: "reveal",
        match: {
          ...state.match,
          verdict: r,
          pending: false,
          deadline: 0,
          selected: r.answer_id ?? state.match.selected,
          marks: nextMarks,
          score: r.score,
        },
      };
    }
    case "opponent.progress": {
      if (!state.match) return state;
      return {
        ...state,
        match: {
          ...state.match,
          opponent: {
            ...state.match.opponent,
            answered: Math.max(state.match.opponent.answered, num(d.answered)),
            finished: state.match.opponent.finished || d.finished === true,
          },
        },
      };
    }
    case "opponent.status": {
      if (!state.match) return state;
      const connected = d.state !== "disconnected";
      return {
        ...state,
        notice: {
          tone: connected ? "info" : "error",
          key: connected ? "opponentBack" : "opponentDisconnected",
        },
        match: { ...state.match, opponent: { ...state.match.opponent, connected } },
      };
    }
    case "match.waiting":
      if (!state.match) return state;
      return { ...state, phase: "waiting", match: { ...state.match, question: null, deadline: 0 } };
    case "match.state": {
      const opp = (d.opponent ?? {}) as Dict;
      const phase = str(d.phase);
      const question = isPlayableQuestion(d.question) ? d.question : null;
      const verdict = asResult(d.last_result);
      const m = mode(d.mode);
      const next: MatchView = {
        id: str(d.match_id),
        mode: m,
        total: num(d.total, 10),
        questionTimeMs: num(d.question_time_ms, 15000),
        startsAt: toLocal(num(d.starts_at_ms), d, at),
        you: card(d.you),
        opponent: {
          ...card(opp),
          answered: num(opp.answered),
          finished: opp.finished === true,
          connected: opp.connected !== false,
        },
        index: num(d.index),
        question,
        deadline: phase === "question" ? toLocal(num(d.deadline_ms), d, at) : 0,
        selected: verdict?.answer_id ?? null,
        pending: false,
        verdict: phase === "reveal" ? verdict : null,
        marks: marks(d.marks),
        score: num(d.score),
      };
      const screen: ArenaPhase =
        phase === "countdown" || phase === "reveal" || phase === "waiting"
          ? phase
          : question
            ? "question"
            : "waiting";
      return { ...state, phase: screen, match: next, notice: null, lastMode: m };
    }
    case "match.end": {
      const sc = (d.score ?? {}) as Dict;
      const co = (d.correct ?? {}) as Dict;
      const mk = (d.marks ?? {}) as Dict;
      const outcome = d.outcome === "won" || d.outcome === "lost" ? d.outcome : "draw";
      return {
        ...state,
        phase: "result",
        notice: null,
        match: null,
        result: {
          mode: mode(d.mode),
          outcome,
          reason: str(d.reason),
          score: { you: num(sc.you), opponent: num(sc.opponent) },
          correct: { you: num(co.you), opponent: num(co.opponent) },
          marks: { you: marks(mk.you), opponent: marks(mk.opponent) },
          total: num(d.total, state.match?.total ?? 10),
          rated: d.rated === true,
          ratingBefore: num(d.rating_before),
          ratingAfter: num(d.rating_after),
          ratingDelta: num(d.rating_delta),
          medal: str(d.medal, "bronze"),
          you: state.match?.you ?? { name: "", rating: num(d.rating_after), medal: str(d.medal) },
          opponent: state.match?.opponent ?? { name: "", rating: 0, medal: "bronze" },
        },
      };
    }
    case "error": {
      const code = str(d.code);
      if (ANSWER_ERRORS.has(code)) {
        return state.match ? { ...state, match: { ...state.match, pending: false } } : state;
      }
      const key = ERROR_NOTICE[code] ?? "serverBusy";
      if (code === "already_queued") return state;
      return {
        ...state,
        phase:
          code === "vip_required"
            ? "error"
            : state.phase === "searching" || state.phase === "connecting"
              ? "lobby"
              : state.phase,
        notice: { tone: "error", key },
      };
    }
    default:
      return state;
  }
}

export function arenaReducer(state: ArenaState, action: ArenaAction): ArenaState {
  switch (action.type) {
    case "server":
      return onServer(state, action.env, action.at);
    case "connecting":
      return state.phase === "idle" || state.phase === "error"
        ? { ...state, phase: "connecting" }
        : state;
    case "socketClosed": {
      if (action.replaced) {
        return { ...state, phase: "error", notice: { tone: "error", key: "replaced" }, invite: null };
      }
      if (action.willRetry) {
        // Keep the duel on screen: the server holds the seat for the
        // reconnect grace and resyncs us with match.state.
        return {
          ...state,
          phase: isInMatch(state.phase) || state.phase === "result" ? state.phase : "connecting",
          notice: { tone: "info", key: "reconnecting" },
          invite: null,
        };
      }
      return {
        ...state,
        phase: state.phase === "result" ? "result" : "error",
        notice: { tone: "error", key: "connectError" },
        invite: null,
      };
    }
    case "connectFailed":
      return { ...state, phase: "error", notice: { tone: "error", key: "connectError" } };
    case "selectAnswer": {
      const m = state.match;
      if (state.phase !== "question" || !m || m.pending) return state;
      return { ...state, match: { ...m, selected: action.answerId, pending: true } };
    }
    case "answerNotSent":
      return state.match
        ? {
            ...state,
            match: { ...state.match, selected: null, pending: false },
            notice: { tone: "error", key: "reconnecting" },
          }
        : state;
    case "searchCancelled":
      return state.phase === "searching" ? { ...state, phase: "lobby" } : state;
    case "backToLobby":
      return { ...state, phase: "lobby", result: null, match: null, notice: null };
    case "dismissNotice":
      return { ...state, notice: null };
    default:
      return state;
  }
}
