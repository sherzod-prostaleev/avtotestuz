import { render, screen } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { describe, expect, it, vi } from "vitest";
import messages from "../../../messages/uz-Latn.json";
import { OfficialAvtotestExamView, examVisual } from "./official-avtotest-exam-view";
import type { SessionQuestionItem, SessionState } from "@/hooks/use-session-engine";

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
}));

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

function examSession(partial: Partial<SessionState> = {}): SessionState {
  return {
    id: "sess-1",
    mode: "exam",
    time_limit_sec: 1500,
    remaining_sec: 1500,
    status: "active",
    questions: [q()],
    score: null,
    total: 20,
    stopped_reason: null,
    passed: null,
    completed_at: null,
    errors_allowed: 2,
    ...partial,
  };
}

function renderExam(session: SessionState) {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      <OfficialAvtotestExamView
        session={session}
        currentIndex={0}
        onSelectIndex={() => {}}
        onSelectAnswer={() => {}}
        onFinish={() => {}}
        submitting={false}
        finishing={false}
        exitHref="/uz-Latn/dashboard"
      />
    </NextIntlClientProvider>
  );
}

describe("OfficialAvtotestExamView error budget HUD", () => {
  it("shows the session's own budget, not one guessed from the mode", () => {
    renderExam(examSession({ errors_allowed: 4, total: 50 }));
    expect(screen.getByText("Xato 0/4")).toBeInTheDocument();
  });

  it("counts wrong answers against that budget", () => {
    renderExam(
      examSession({
        errors_allowed: 4,
        total: 50,
        questions: [
          q({ answered: true, user_answer_id: "a-2", correct: false }),
          q({ id: "q-2", answered: true, user_answer_id: "a-1", correct: true }),
        ],
      })
    );
    expect(screen.getByText("Xato 1/4")).toBeInTheDocument();
  });

  // Sessions created before the backend reported the column carry null.
  it("falls back to the standard budget when the server did not send one", () => {
    renderExam(examSession({ errors_allowed: null }));
    expect(screen.getByText("Xato 0/2")).toBeInTheDocument();
  });

  it("keeps placement's tighter budget when the server did not send one", () => {
    renderExam(examSession({ mode: "placement", errors_allowed: null, total: 10 }));
    expect(screen.getByText("Xato 0/1")).toBeInTheDocument();
  });
});
