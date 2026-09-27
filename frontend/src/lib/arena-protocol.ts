/** Arena wire protocol types (M4-03 §2.4) — client side. */

export type ArenaEnvelope = {
  v: number;
  t: string;
  d: unknown;
};

export type ArenaPhase =
  | "idle"
  | "connecting"
  | "lobby"
  | "searching"
  | "countdown"
  | "question"
  | "reveal"
  | "waiting"
  | "result"
  | "error";

/** ranked moves rating; friend (invite code) and bot practice do not. */
export type ArenaMode = "ranked" | "friend" | "bot";

/** One question's verdict for one player, as the progress bars draw it. */
export type RoundMark = "correct" | "wrong" | "skipped";

export type PlayerCard = {
  name: string;
  rating: number;
  medal: string;
  bot?: boolean;
};

export type QuestionPayload = {
  id: string;
  text: string;
  image_url?: string | null;
  answers: { id: string; position: number; text: string; image_url?: string | null }[];
};

/** Verdict on the player's own question (`answer.result`). */
export type AnswerResult = {
  index: number;
  answered: boolean;
  correct: boolean;
  answer_id?: string;
  correct_answer_id: string;
  points: number;
  score: number;
  response_ms: number;
  next_in_ms: number;
  last: boolean;
};

/** True when a WS question payload is safe to render (avoids error-boundary crash). */
export function isPlayableQuestion(q: unknown): q is QuestionPayload {
  if (!q || typeof q !== "object") return false;
  const obj = q as Partial<QuestionPayload>;
  if (typeof obj.id !== "string" || !obj.id) return false;
  if (!Array.isArray(obj.answers) || obj.answers.length === 0) return false;
  return obj.answers.every(
    (a) =>
      a &&
      typeof a === "object" &&
      typeof a.id === "string" &&
      typeof a.text === "string"
  );
}

export function parseEnvelope(raw: string): ArenaEnvelope {
  const env = JSON.parse(raw) as ArenaEnvelope;
  if (env.v !== 1 || typeof env.t !== "string") {
    throw new Error("bad_protocol");
  }
  return env;
}

export function encodeClient(t: string, d: Record<string, unknown> = {}): string {
  return JSON.stringify({ v: 1, t, d });
}

export function medalLabel(medal: string): string {
  switch (medal) {
    case "brilliant":
      return "Brilliant";
    case "diamond":
      return "Diamond";
    case "platinum":
      return "Platinum";
    case "gold":
      return "Gold";
    case "silver":
      return "Silver";
    default:
      return "Bronze";
  }
}

/** Mirrors the server's alphabet: no 0/O or 1/I/L, six characters. */
const INVITE_ALPHABET = "ABCDEFGHJKMNPQRSTUVWXYZ23456789";
export const INVITE_CODE_LEN = 6;

/** What a person types or pastes → a code, or "" while it cannot be one yet. */
export function normalizeInviteCode(raw: string): string {
  let out = "";
  for (const ch of raw.toUpperCase()) {
    if (ch === " " || ch === "-") continue;
    if (!INVITE_ALPHABET.includes(ch)) return "";
    out += ch;
  }
  return out.length === INVITE_CODE_LEN ? out : "";
}

/** Keeps only characters a code can hold, for the input as the user types. */
export function sanitizeInviteInput(raw: string): string {
  let out = "";
  for (const ch of raw.toUpperCase()) {
    if (INVITE_ALPHABET.includes(ch)) out += ch;
    if (out.length === INVITE_CODE_LEN) break;
  }
  return out;
}
