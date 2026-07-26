import { describe, expect, it } from "vitest";
import { examVisual } from "./official-avtotest-exam-view";
import type { SessionQuestionItem } from "@/hooks/use-session-engine";

function q(partial: Partial<SessionQuestionItem> = {}): SessionQuestionItem {
  return {
    id: "q-1",
    question: "Savol?",
    answers: [
      { id: "a-1", text: "A" },
      { id: "a-2", text: "B" },
    ],
    ...partial,
  };
}

describe("examVisual", () => {
  it("paints the chosen correct answer green, never blue", () => {
    const question = q({
      answered: true,
      user_answer_id: "a-1",
      correct: true,
      correct_answer_id: "a-1",
    });
    expect(examVisual(question, "a-1")).toBe("correct");
    expect(examVisual(question, "a-2")).toBe("neutral");
  });

  it("paints a wrong choice red without revealing the key", () => {
    const question = q({
      answered: true,
      user_answer_id: "a-2",
      correct: false,
      correct_answer_id: "a-1",
    });
    expect(examVisual(question, "a-2")).toBe("wrong");
    expect(examVisual(question, "a-1")).toBe("neutral");
  });

  it("uses brief blue only while the submit is pending", () => {
    const question = q();
    expect(examVisual(question, "a-1", { questionId: "q-1", answerId: "a-1" })).toBe("selected");
    expect(examVisual(question, "a-2", { questionId: "q-1", answerId: "a-1" })).toBe("neutral");
  });

  it("does not keep answered-without-grade as blue", () => {
    const question = q({ answered: true, user_answer_id: "a-1" });
    expect(examVisual(question, "a-1")).toBe("answered");
  });
});
