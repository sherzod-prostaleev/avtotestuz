import { describe, expect, it } from "vitest";
import { arenaReducer, initialArenaState, type ArenaState } from "@/lib/arena-state";
import type { ArenaEnvelope } from "@/lib/arena-protocol";

const q = (id: string) => ({
  id,
  text: "Savol",
  answers: [
    { id: `${id}-a`, position: 1, text: "A" },
    { id: `${id}-b`, position: 2, text: "B" },
  ],
});

function feed(state: ArenaState, t: string, d: unknown, at = 1_000_000): ArenaState {
  const env: ArenaEnvelope = { v: 1, t, d };
  return arenaReducer(state, { type: "server", env, at });
}

function inDuel(): ArenaState {
  let s = feed(initialArenaState, "hello", { online: 3 });
  s = feed(s, "match.found", {
    match_id: "m1",
    mode: "ranked",
    you: { name: "Siz", rating: 1010, medal: "bronze" },
    opponent: { name: "Ali", rating: 990, medal: "bronze" },
    question_count: 10,
    question_time_ms: 15000,
    starts_at_ms: 1_003_000,
    server_time_ms: 1_000_000,
  });
  return feed(s, "question", {
    index: 0,
    total: 10,
    deadline_ms: 1_018_000,
    server_time_ms: 1_003_000,
    question: q("q0"),
  }, 1_003_000);
}

describe("arenaReducer", () => {
  it("goes from hello to the lobby and remembers who is online", () => {
    const s = feed(initialArenaState, "hello", { online: 7 });
    expect(s.phase).toBe("lobby");
    expect(s.online).toBe(7);
  });

  it("names the opponent and their rating when a match is found", () => {
    let s = feed(initialArenaState, "hello", { online: 2 });
    s = feed(s, "match.found", {
      match_id: "m1",
      mode: "bot",
      you: { name: "Siz", rating: 1000, medal: "bronze" },
      opponent: { name: "AvtoBot", rating: 1000, medal: "bronze", bot: true },
      question_count: 10,
      starts_in_ms: 3000,
    });
    expect(s.phase).toBe("countdown");
    expect(s.match?.opponent).toMatchObject({ name: "AvtoBot", bot: true, answered: 0 });
    expect(s.match?.mode).toBe("bot");
    expect(s.lastMode).toBe("bot");
  });

  it("shifts server deadlines onto the local clock", () => {
    // The phone's clock is 5 s behind the server.
    let s = feed(initialArenaState, "hello", {});
    s = feed(s, "match.found", { match_id: "m", starts_at_ms: 0 }, 0);
    s = feed(s, "question", {
      index: 0,
      deadline_ms: 20_000,
      server_time_ms: 5_000,
      question: q("q0"),
    }, 0);
    expect(s.match?.deadline).toBe(15_000);
  });

  it("marks a verdict immediately and never waits for the opponent", () => {
    let s = inDuel();
    s = arenaReducer(s, { type: "selectAnswer", answerId: "q0-a" });
    expect(s.match?.pending).toBe(true);
    // A second tap while the first is in flight is ignored.
    expect(arenaReducer(s, { type: "selectAnswer", answerId: "q0-b" }).match?.selected).toBe("q0-a");
    s = feed(s, "answer.result", {
      index: 0,
      answered: true,
      correct: true,
      answer_id: "q0-a",
      correct_answer_id: "q0-a",
      points: 87,
      score: 87,
      next_in_ms: 1200,
    });
    expect(s.phase).toBe("reveal");
    expect(s.match?.marks).toEqual(["correct"]);
    expect(s.match?.score).toBe(87);
    s = feed(s, "question", { index: 1, deadline_ms: 1_020_000, server_time_ms: 1_005_000, question: q("q1") });
    expect(s.phase).toBe("question");
    expect(s.match?.index).toBe(1);
    expect(s.match?.selected).toBeNull();
    expect(s.match?.marks).toEqual(["correct"]);
  });

  it("records a timeout as skipped", () => {
    const s = feed(inDuel(), "answer.result", {
      index: 0,
      answered: false,
      correct: false,
      correct_answer_id: "q0-a",
      points: 0,
      score: 0,
    });
    expect(s.match?.marks).toEqual(["skipped"]);
  });

  it("moves the opponent's bar forward only", () => {
    let s = feed(inDuel(), "opponent.progress", { answered: 3, total: 10 });
    expect(s.match?.opponent.answered).toBe(3);
    s = feed(s, "opponent.progress", { answered: 2, total: 10 });
    expect(s.match?.opponent.answered).toBe(3);
    s = feed(s, "opponent.progress", { answered: 10, total: 10, finished: true });
    expect(s.match?.opponent.finished).toBe(true);
  });

  it("waits after finishing first", () => {
    const s = feed(inDuel(), "match.waiting", { score: 500, correct: 7 });
    expect(s.phase).toBe("waiting");
    expect(s.match?.question).toBeNull();
  });

  it("rebuilds a duel from a resync after a reload", () => {
    let s = feed(initialArenaState, "hello", { online: 1, in_match: true });
    expect(s.phase).toBe("idle");
    s = feed(s, "match.state", {
      match_id: "m1",
      mode: "friend",
      phase: "question",
      index: 4,
      total: 10,
      deadline_ms: 1_010_000,
      server_time_ms: 1_000_000,
      question: q("q4"),
      you: { name: "Siz", rating: 1000, medal: "bronze" },
      marks: ["correct", "wrong", "skipped", "correct"],
      score: 180,
      opponent: { name: "Ali", rating: 990, medal: "bronze", answered: 6, finished: false, connected: true },
    });
    expect(s.phase).toBe("question");
    expect(s.match?.index).toBe(4);
    expect(s.match?.marks).toHaveLength(4);
    expect(s.match?.opponent.answered).toBe(6);
    expect(s.match?.deadline).toBe(1_010_000);
  });

  it("tells a player whose match ended while they were offline", () => {
    let s = inDuel();
    s = arenaReducer(s, { type: "socketClosed", replaced: false, willRetry: true });
    expect(s.phase).toBe("question");
    expect(s.notice?.key).toBe("reconnecting");
    s = feed(s, "hello", { online: 1, in_match: false });
    expect(s.phase).toBe("lobby");
    expect(s.match).toBeNull();
    expect(s.notice?.key).toBe("matchEndedOffline");
  });

  it("stops at an error screen when another tab took over", () => {
    const s = arenaReducer(inDuel(), { type: "socketClosed", replaced: true, willRetry: false });
    expect(s.phase).toBe("error");
    expect(s.notice?.key).toBe("replaced");
  });

  it("shows the full result with both players' marks", () => {
    const s = feed(inDuel(), "match.end", {
      mode: "ranked",
      outcome: "won",
      reason: "completed",
      score: { you: 640, opponent: 410 },
      correct: { you: 8, opponent: 6 },
      marks: { you: ["correct", "wrong"], opponent: ["skipped", "correct"] },
      total: 10,
      rated: true,
      rating_before: 1010,
      rating_after: 1026,
      rating_delta: 16,
      medal: "bronze",
    });
    expect(s.phase).toBe("result");
    expect(s.match).toBeNull();
    expect(s.result).toMatchObject({
      outcome: "won",
      rated: true,
      ratingDelta: 16,
      opponent: expect.objectContaining({ name: "Ali" }),
    });
    expect(s.result?.marks.opponent).toEqual(["skipped", "correct"]);
  });

  it("returns a failed search to the lobby with the reason", () => {
    let s = feed(initialArenaState, "hello", {});
    s = feed(s, "queue.joined", { queued_at_ms: 1, timeout_ms: 45000, online: 1 });
    expect(s.phase).toBe("searching");
    s = feed(s, "queue.timeout", { waited_ms: 45000, online: 1 });
    expect(s.phase).toBe("lobby");
    expect(s.notice?.key).toBe("timeout");
  });

  it("clears a pending answer when the server rejects it", () => {
    let s = arenaReducer(inDuel(), { type: "selectAnswer", answerId: "q0-a" });
    s = feed(s, "error", { code: "too_late" });
    expect(s.match?.pending).toBe(false);
    expect(s.notice).toBeNull();
  });

  it("maps invite errors to their own notices", () => {
    const s = feed(feed(initialArenaState, "hello", {}), "error", { code: "invite_host_away" });
    expect(s.phase).toBe("lobby");
    expect(s.notice?.key).toBe("inviteHostAway");
  });

  it("gives a reconnecting host their open invite code back", () => {
    let s = feed(feed(initialArenaState, "hello", {}), "invite.created", { code: "ABC234", expires_in_sec: 600 });
    s = arenaReducer(s, { type: "socketClosed", replaced: false, willRetry: true });
    expect(s.invite).toBeNull();
    s = feed(s, "hello", { online: 2, invite: { code: "ABC234", expires_in_sec: 420 } });
    expect(s.invite?.code).toBe("ABC234");
  });
});
