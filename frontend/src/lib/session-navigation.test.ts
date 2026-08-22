import { describe, expect, it } from "vitest";
import type { SessionQuestionItem } from "@/hooks/use-session-engine";
import { AUTO_ADVANCE_MS, hasAnswer, nextUnansweredIndex } from "@/lib/session-navigation";

function q(id: string, answered = false): SessionQuestionItem {
  return {
    id,
    question: `savol ${id}`,
    image_url: null,
    answers: [{ id: `${id}-a`, text: "javob" }],
    answered,
    user_answer_id: answered ? `${id}-a` : null,
  };
}

describe("hasAnswer", () => {
  it("counts a recorded answer id even when the answered flag is missing", () => {
    expect(hasAnswer({ ...q("1"), answered: undefined, user_answer_id: "x" })).toBe(true);
  });

  it("is false for an untouched question", () => {
    expect(hasAnswer(q("1"))).toBe(false);
  });
});

describe("nextUnansweredIndex", () => {
  it("moves to the next question when it is still unanswered", () => {
    expect(nextUnansweredIndex([q("1"), q("2"), q("3")], 0)).toBe(1);
  });

  it("skips over questions already answered", () => {
    expect(nextUnansweredIndex([q("1"), q("2", true), q("3")], 0)).toBe(2);
  });

  it("wraps back to an earlier gap when nothing is left ahead", () => {
    expect(nextUnansweredIndex([q("1"), q("2", true), q("3", true)], 1)).toBe(0);
  });

  it("returns -1 when every question is answered", () => {
    expect(nextUnansweredIndex([q("1", true), q("2", true)], 0)).toBe(-1);
  });

  // The caller schedules the hop straight after a submit, while the question
  // just answered may not have landed in the array yet.
  it("treats the just-answered question as answered", () => {
    expect(nextUnansweredIndex([q("1"), q("2", true)], 0, "1")).toBe(-1);
  });

  it("still finds a later gap after the just-answered question", () => {
    expect(nextUnansweredIndex([q("1"), q("2"), q("3")], 0, "1")).toBe(1);
  });

  it("stays put when there are no questions at all", () => {
    expect(nextUnansweredIndex([], 0)).toBe(-1);
  });
});

describe("AUTO_ADVANCE_MS", () => {
  it("holds the graded answer on screen long enough to read", () => {
    expect(AUTO_ADVANCE_MS).toBe(900);
  });
});
