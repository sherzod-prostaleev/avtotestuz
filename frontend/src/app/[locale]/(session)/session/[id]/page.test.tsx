import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { beforeEach, describe, expect, it, vi } from "vitest";
import messages from "../../../../../../messages/uz-Latn.json";
import SessionPage from "./page";
import {
  useSessionEngine,
  type SessionQuestionItem,
  type SessionState,
} from "@/hooks/use-session-engine";
import * as apiClient from "@/lib/api-client";
import { trackEvent } from "@/lib/analytics-events";

const navigation = vi.hoisted(() => ({ push: vi.fn() }));

vi.mock("next/navigation", () => ({
  useParams: () => ({ id: "sess-123" }),
  useRouter: () => ({ push: navigation.push, replace: vi.fn() }),
}));

vi.mock("@/hooks/use-session-engine", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/hooks/use-session-engine")>();
  return { ...actual, useSessionEngine: vi.fn() };
});

vi.mock("@/lib/analytics-events", () => ({ trackEvent: vi.fn() }));

function question(overrides: Partial<SessionQuestionItem> = {}): SessionQuestionItem {
  return {
    id: "q-1",
    question: "Qaysi belgi to'xtashni taqiqlaydi?",
    image_url: null,
    answers: [
      { id: "a-1", text: "3.27 belgisi" },
      { id: "a-2", text: "3.28 belgisi" },
    ],
    answered: false,
    user_answer_id: null,
    ...overrides,
  };
}

function activeSession(overrides: Partial<SessionState> = {}): SessionState {
  return {
    id: "sess-123",
    mode: "variant",
    time_limit_sec: null,
    remaining_sec: null,
    status: "active",
    questions: [question()],
    score: null,
    total: 1,
    stopped_reason: null,
    passed: null,
    completed_at: null,
    ...overrides,
  };
}

function mockEngine(
  session: SessionState | null,
  overrides: Partial<ReturnType<typeof useSessionEngine>> = {}
) {
  const engine: ReturnType<typeof useSessionEngine> = {
    session,
    loading: false,
    submitting: false,
    error: null,
    loadSession: vi.fn().mockResolvedValue(session),
    startSession: vi.fn(),
    submitAnswer: vi.fn().mockResolvedValue({ recorded: true, correct: true }),
    finishSession: vi.fn().mockResolvedValue(session),
    ...overrides,
  };
  vi.mocked(useSessionEngine).mockReturnValue(engine);
  return engine;
}

function renderPage() {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      <SessionPage />
    </NextIntlClientProvider>
  );
}

describe("SessionPage secure session flow", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    navigation.push.mockReset();
    vi.mocked(useSessionEngine).mockReset();
    vi.mocked(trackEvent).mockReset();
    vi.spyOn(apiClient, "apiGet").mockResolvedValue([] as never);
    vi.spyOn(apiClient, "apiPost").mockResolvedValue({ ok: true } as never);
    vi.spyOn(apiClient, "apiDelete").mockResolvedValue({ ok: true } as never);
  });

  it("loads the session with the current locale and renders the server question", async () => {
    const engine = mockEngine(
      activeSession({
        questions: [
          question(),
          question({ id: "q-2", question: "Ikkinchi savol" }),
        ],
        total: 2,
      })
    );

    renderPage();

    expect(screen.getByText("Qaysi belgi to'xtashni taqiqlaydi?")).toBeInTheDocument();
    expect(screen.getByText("3.27 belgisi")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Keyingisi" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "1-savol: joriy" })).toHaveAttribute(
      "aria-current",
      "step"
    );
    await waitFor(() => expect(engine.loadSession).toHaveBeenCalledWith("sess-123", "uz-Latn"));
    expect(trackEvent).toHaveBeenCalledWith("view_question", {
      session_id: "sess-123",
      question_id: "q-1",
      mode: "variant",
      position: 1,
      locale: "uz-Latn",
    });
  });

  it("shows persisted feedback using only backend correctness fields", () => {
    mockEngine(
      activeSession({
        questions: [
          question({
            answered: true,
            user_answer_id: "a-1",
            correct: false,
            correct_answer_id: "a-2",
            explanation: {
              blocks: [{ type: "muhim", content: "YHQ 91-band bo'yicha tekshirilgan izoh." }],
            },
          }),
          question({ id: "q-2", question: "Ikkinchi savol" }),
        ],
        total: 2,
      })
    );

    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "1-savol: xato" }));

    expect(screen.getByTestId("answer-incorrect-icon")).toBeInTheDocument();
    expect(screen.getByTestId("answer-correct-icon")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Keyingisi" })).toBeEnabled();

    // The explanation must stay out of the answering flow so the stage keeps a fixed height.
    expect(screen.queryByText("YHQ 91-band bo'yicha tekshirilgan izoh.")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Ekspert tahlili" }));
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    expect(screen.getByText("YHQ 91-band bo'yicha tekshirilgan izoh.")).toBeVisible();
  });

  it("paints a graded exam answer green and disables further choice", () => {
    mockEngine(
      activeSession({
        mode: "exam",
        time_limit_sec: 1500,
        remaining_sec: 300,
        questions: [
          question({
            answered: true,
            user_answer_id: "a-1",
            correct: true,
            correct_answer_id: "a-1",
          }),
        ],
      })
    );

    renderPage();

    const chosen = screen.getByRole("button", { name: /3.27 belgisi/ });
    expect(chosen).toBeDisabled();
    expect(chosen.className).toMatch(/border-green-500/);
    expect(chosen.className).not.toMatch(/border-blue-400/);
  });

  it("submits a dynamic fifth answer through F5 without client grading", async () => {
    const answers = Array.from({ length: 5 }, (_, index) => ({
      id: `a-${index + 1}`,
      text: `${index + 1}-javob`,
    }));
    const submitAnswer = vi.fn().mockResolvedValue({ recorded: true });
    mockEngine(activeSession({ questions: [question({ answers })] }), { submitAnswer });

    renderPage();
    fireEvent.keyDown(window, { key: "F5" });

    await waitFor(() =>
      expect(submitAnswer).toHaveBeenCalledWith(
        "sess-123",
        "q-1",
        "a-5",
        expect.objectContaining({ latencyMs: expect.any(Number), skipFsrs: true })
      )
    );
    expect(trackEvent).toHaveBeenCalledWith(
      "answer",
      expect.objectContaining({ session_id: "sess-123", question_id: "q-1", status: "recorded" })
    );
  });

  it("shows optimistic selected state while answer submit is in flight", async () => {
    let resolveSubmit: (value: { recorded: boolean; correct: boolean; correct_answer_id: string }) => void =
      () => undefined;
    const submitAnswer = vi.fn(
      () =>
        new Promise<{ recorded: boolean; correct: boolean; correct_answer_id: string }>((resolve) => {
          resolveSubmit = resolve;
        })
    );
    mockEngine(activeSession({ mode: "practice" }), { submitAnswer, submitting: false });

    renderPage();
    fireEvent.click(screen.getByRole("button", { name: /3.28 belgisi/ }));

    await waitFor(() =>
      expect(submitAnswer).toHaveBeenCalledWith(
        "sess-123",
        "q-1",
        "a-2",
        expect.objectContaining({ latencyMs: expect.any(Number), skipFsrs: true })
      )
    );
    // Pending selection must paint immediately — no neutral→dim→wrong flash.
    expect(screen.getByRole("button", { name: /3.28 belgisi/ }).className).toMatch(/border-accent/);
    expect(screen.queryByTestId("answer-incorrect-icon")).not.toBeInTheDocument();

    resolveSubmit({ recorded: true, correct: false, correct_answer_id: "a-1" });
  });

  it("posts Again to learn/review immediately on incorrect deferred-FSRS answer", async () => {
    const submitAnswer = vi.fn().mockResolvedValue({
      recorded: true,
      correct: false,
      correct_answer_id: "a-1",
    });
    mockEngine(activeSession({ mode: "review" }), { submitAnswer });
    const post = vi.spyOn(apiClient, "apiPost").mockResolvedValue({ ok: true } as never);

    renderPage();
    fireEvent.click(screen.getByRole("button", { name: /3.28 belgisi/ }));

    await waitFor(() =>
      expect(post).toHaveBeenCalledWith("learn/review", { question_id: "q-1", rating: 1 })
    );
    expect(submitAnswer).toHaveBeenCalledWith(
      "sess-123",
      "q-1",
      "a-2",
      expect.objectContaining({ skipFsrs: true })
    );
  });

  it("shows Hard/Good/Easy after a correct deferred answer and posts the pick", async () => {
    let graded = false;
    const gradedSession = () =>
      activeSession({
        mode: "variant",
        questions: [
          question(
            graded
              ? {
                  answered: true,
                  user_answer_id: "a-1",
                  correct: true,
                  correct_answer_id: "a-1",
                }
              : {}
          ),
          question({ id: "q-2", question: "Ikkinchi savol" }),
        ],
        total: 2,
      });

    const submitAnswer = vi.fn().mockImplementation(async () => {
      graded = true;
      const next = {
        session: gradedSession(),
        loading: false,
        submitting: false,
        error: null,
        loadSession: vi.fn().mockResolvedValue(gradedSession()),
        startSession: vi.fn(),
        submitAnswer,
        finishSession: vi.fn().mockResolvedValue(gradedSession()),
      };
      vi.mocked(useSessionEngine).mockReturnValue(next);
      return { recorded: true, correct: true, correct_answer_id: "a-1" };
    });
    mockEngine(gradedSession(), { submitAnswer });
    const post = vi.spyOn(apiClient, "apiPost").mockResolvedValue({ ok: true } as never);

    const { rerender } = renderPage();
    fireEvent.click(screen.getByRole("button", { name: /3.27 belgisi/ }));

    await waitFor(() => expect(submitAnswer).toHaveBeenCalled());
    rerender(
      <NextIntlClientProvider locale="uz-Latn" messages={messages}>
        <SessionPage />
      </NextIntlClientProvider>
    );

    expect(screen.getByText("Takrorlash: tez javob → oson; sekin → qiyin")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Oson" }));
    await waitFor(() =>
      expect(post).toHaveBeenCalledWith("learn/review", { question_id: "q-1", rating: 4 })
    );
  });

  it("defaults pending FSRS to Good when advancing with Keyingisi", async () => {
    let graded = false;
    const gradedSession = () =>
      activeSession({
        mode: "practice",
        questions: [
          question(
            graded
              ? {
                  answered: true,
                  user_answer_id: "a-1",
                  correct: true,
                  correct_answer_id: "a-1",
                }
              : {}
          ),
          question({ id: "q-2", question: "Ikkinchi savol" }),
        ],
        total: 2,
      });
    const submitAnswer = vi.fn().mockImplementation(async () => {
      graded = true;
      vi.mocked(useSessionEngine).mockReturnValue({
        session: gradedSession(),
        loading: false,
        submitting: false,
        error: null,
        loadSession: vi.fn().mockResolvedValue(gradedSession()),
        startSession: vi.fn(),
        submitAnswer,
        finishSession: vi.fn().mockResolvedValue(gradedSession()),
      });
      return { recorded: true, correct: true, correct_answer_id: "a-1" };
    });
    mockEngine(gradedSession(), { submitAnswer });
    const post = vi.spyOn(apiClient, "apiPost").mockResolvedValue({ ok: true } as never);

    const { rerender } = renderPage();
    fireEvent.click(screen.getByRole("button", { name: /3.27 belgisi/ }));
    await waitFor(() => expect(submitAnswer).toHaveBeenCalled());
    rerender(
      <NextIntlClientProvider locale="uz-Latn" messages={messages}>
        <SessionPage />
      </NextIntlClientProvider>
    );

    fireEvent.click(screen.getByRole("button", { name: "Keyingisi" }));
    await waitFor(() =>
      expect(post).toHaveBeenCalledWith("learn/review", { question_id: "q-1", rating: 3 })
    );
  });

  it("exam mode submits without skip_fsrs", async () => {
    const submitAnswer = vi.fn().mockResolvedValue({
      recorded: true,
      correct: true,
      correct_answer_id: "a-1",
    });
    mockEngine(
      activeSession({
        mode: "exam",
        time_limit_sec: 1500,
        remaining_sec: 300,
        questions: [question()],
      }),
      { submitAnswer }
    );

    renderPage();
    fireEvent.click(screen.getByRole("button", { name: /3.27 belgisi/ }));

    await waitFor(() =>
      expect(submitAnswer).toHaveBeenCalledWith(
        "sess-123",
        "q-1",
        "a-1",
        expect.objectContaining({ latencyMs: expect.any(Number) })
      )
    );
    const opts = submitAnswer.mock.calls[0]?.[3] as { skipFsrs?: boolean } | undefined;
    expect(opts?.skipFsrs).toBeFalsy();
    expect(screen.queryByRole("button", { name: "Yaxshi" })).not.toBeInTheDocument();
  });

  it("persists a bookmark before changing its visible state", async () => {
    mockEngine(activeSession());
    const post = vi.spyOn(apiClient, "apiPost").mockResolvedValue({ ok: true } as never);

    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "Savolni saqlash" }));

    await waitFor(() =>
      expect(post).toHaveBeenCalledWith("me/saved", { question_id: "q-1" })
    );
    expect(screen.getByRole("button", { name: "Saqlanganlardan olib tashlash" })).toHaveAttribute(
      "aria-pressed",
      "true"
    );
  });

  it("renders a calm completed result and per-question review", () => {
    mockEngine(
      activeSession({
        mode: "exam",
        status: "completed",
        score: 1,
        total: 1,
        passed: true,
        completed_at: "2026-07-22T12:00:00Z",
        questions: [
          question({
            answered: true,
            user_answer_id: "a-1",
            correct: true,
            correct_answer_id: "a-1",
            explanation: {
              blocks: [{ type: "muhim", content: "Rasmiy yakuniy izoh" }],
            },
          }),
        ],
      })
    );

    renderPage();

    expect(screen.getByText("Imtihondan o'tdingiz!")).toBeInTheDocument();
    expect(screen.getByText("1 / 1")).toBeInTheDocument();
    expect(screen.getByText("Savollar bo'yicha tahlil")).toBeInTheDocument();
    expect(screen.getAllByText("3.27 belgisi")).toHaveLength(2);
    fireEvent.click(screen.getByText("Rasmiy izohni ochish"));
    expect(screen.getByText("Rasmiy yakuniy izoh")).toBeInTheDocument();
  });

  it("never exposes a raw backend error message", () => {
    mockEngine(null, {
      error: { code: "internal", message: "database exploded at sessions.go:42" },
    });

    renderPage();

    expect(screen.getByRole("alert")).toHaveTextContent("Amalni bajarib bo'lmadi");
    expect(screen.queryByText(/database exploded/)).not.toBeInTheDocument();
  });
});
