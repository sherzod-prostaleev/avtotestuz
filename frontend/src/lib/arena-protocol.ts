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
  | "result"
  | "error";

export type QuestionPayload = {
  id: string;
  text: string;
  image_url?: string | null;
  answers: { id: string; position: number; text: string; image_url?: string | null }[];
};

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
