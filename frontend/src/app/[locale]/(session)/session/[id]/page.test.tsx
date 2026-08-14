import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { beforeEach, describe, expect, it, vi } from "vitest";
import messages from "../../../../../../messages/uz-Latn.json";
import SessionPage from "./page";
import { PROTECTED_SEGMENTS, matchesAny } from "@/lib/protected-segments";
import {
  useSessionEngine,
  type SessionQuestionItem,
  type SessionState,
} from "@/hooks/use-session-engine";
import * as apiClient from "@/lib/api-client";
import { trackEvent } from "@/lib/analytics-events";
import { QUESTION_IMAGE_PLACEHOLDER } from "@/lib/question-image";

const navigation = vi.hoisted(() => ({ push: vi.fn(), replace: vi.fn() }));

vi.mock("next/navigation", () => ({
  useParams: () => ({ id: "sess-123" }),
  useRouter: () => ({ push: navigation.push, replace: navigation.replace }),
}));

vi.mock("next/dynamic", async () => {
  const { OfficialAvtotestExamView } = await import("@/components/exam/official-avtotest-exam-view");
  const { ExamPassCelebration } = await import("@/components/exam/exam-pass-celebration");
  const { GrandMockCertificateDialog } = await import("@/components/mock/grand-mock-certificate-dialog");
  return {
    default: (loader: () => Promise<{ default: unknown }>) => {
      const src = loader.toString();
      if (src.includes("official-avtotest-exam-view")) return OfficialAvtotestExamView;
      if (src.includes("exam-pass-celebration")) return ExamPassCelebration;
      if (src.includes("grand-mock-certificate-dialog")) return GrandMockCertificateDialog;
      throw new Error(`unmocked next/dynamic loader: ${src}`);
    },
  };
});

vi.mock("@/hooks/use-session-engine", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/hooks/use-session-engine")>();
  return { ...actual, useSessionEngine: vi.fn() };
});

vi.mock("@/lib/analytics-events", () => ({ trackEvent: vi.fn() }));

vi.mock("canvas-confetti", () => ({ default: vi.fn() }));

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

function renderKioskPage() {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      <SessionPage kiosk />
    </NextIntlClientProvider>
  );
}

/** Same check src/proxy.ts runs on every request from a login-free kiosk browser. */
function isKioskReachable(target: string): boolean {
  const withoutLocale = target.replace(/^\/[a-zA-Z-]+/, "");
  const pathname = withoutLocale.split("?")[0] || "/";
  return !matchesAny(pathname, PROTECTED_SEGMENTS);
}

describe("SessionPage secure session flow", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    navigation.push.mockReset();
    navigation.replace.mockReset();
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

  it("shows the Driver Go cars placeholder for a text-only exam question", () => {
    mockEngine(
      activeSession({
        mode: "exam",
        time_limit_sec: 1500,
        remaining_sec: 300,
        questions: [question({ image_url: null })],
      })
    );

    renderPage();

    expect(screen.getByRole("img", { name: "Qaysi belgi to'xtashni taqiqlaydi?" })).toHaveAttribute(
      "src",
      QUESTION_IMAGE_PLACEHOLDER
    );
  });

  it("keeps a real exam diagram when the API sent image_url", () => {
    mockEngine(
      activeSession({
        mode: "exam",
        time_limit_sec: 1500,
        remaining_sec: 300,
        questions: [question({ image_url: "https://media.example.test/3.27.webp" })],
      })
    );

    renderPage();

    expect(screen.getByRole("img", { name: "Qaysi belgi to'xtashni taqiqlaydi?" })).toHaveAttribute(
      "src",
      "https://media.example.test/3.27.webp"
    );
  });

  it("loads a local MinIO diagram via same-origin /media instead of the cars placeholder", () => {
    mockEngine(
      activeSession({
        questions: [
          question({
            image_url:
              "http://localhost:9000/media/images/4567aa175f412cf4822198b6526e414cc38e34947a036e02fcc516bf82b81070.webp",
          }),
        ],
      })
    );

    renderPage();

    expect(screen.getByRole("img", { name: "1-savol rasmi" })).toHaveAttribute(
      "src",
      "/media/images/4567aa175f412cf4822198b6526e414cc38e34947a036e02fcc516bf82b81070.webp"
    );
  });

  it("shows the Driver Go cars placeholder in a bilet when image_url is null", () => {
    mockEngine(activeSession({ questions: [question({ image_url: null })] }));
    renderPage();

    expect(screen.getByRole("img", { name: "1-savol rasmi" })).toHaveAttribute(
      "src",
      QUESTION_IMAGE_PLACEHOLDER
    );
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

  it("submits a dynamic fifth answer through F5 with latency for backend grading", async () => {
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
        expect.objectContaining({ latencyMs: expect.any(Number) })
      )
    );
    const opts = submitAnswer.mock.calls[0]?.[3] as { skipFsrs?: boolean } | undefined;
    expect(opts).not.toHaveProperty("skipFsrs");
    expect(trackEvent).toHaveBeenCalledWith(
      "answer",
      expect.objectContaining({ session_id: "sess-123", question_id: "q-1", status: "recorded" })
    );
  });

  it("shows optimistic selected state while answer submit is in flight", async () => {
    let resolveSubmit: (value: { recorded: boolean; correct: boolean; correct_answer_id: string }) => void =
      () => undefined;
    const submitAnswer = vi.fn(
      (_sessionId: string, _questionId: string, _answerId: string, _options?: unknown) =>
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
        expect.objectContaining({ latencyMs: expect.any(Number) })
      )
    );
    const opts = submitAnswer.mock.calls[0]?.[3] as { skipFsrs?: boolean } | undefined;
    expect(opts).not.toHaveProperty("skipFsrs");
    // Pending selection must paint immediately — no neutral→dim→wrong flash.
    expect(screen.getByRole("button", { name: /3.28 belgisi/ }).className).toMatch(/border-accent/);
    expect(screen.queryByTestId("answer-incorrect-icon")).not.toBeInTheDocument();

    resolveSubmit({ recorded: true, correct: false, correct_answer_id: "a-1" });
  });

  it("delegates an incorrect review answer to SubmitAnswer without posting learn/review", async () => {
    const submitAnswer = vi.fn().mockResolvedValue({
      recorded: true,
      correct: false,
      correct_answer_id: "a-1",
    });
    mockEngine(activeSession({ mode: "review" }), { submitAnswer });
    const post = vi.spyOn(apiClient, "apiPost").mockResolvedValue({ ok: true } as never);

    renderPage();
    fireEvent.click(screen.getByRole("button", { name: /3.28 belgisi/ }));

    await waitFor(() => expect(submitAnswer).toHaveBeenCalled());
    const opts = submitAnswer.mock.calls[0]?.[3] as
      | { latencyMs?: number; skipFsrs?: boolean }
      | undefined;
    expect(opts?.latencyMs).toEqual(expect.any(Number));
    expect(opts).not.toHaveProperty("skipFsrs");
    expect(post).not.toHaveBeenCalledWith("learn/review", expect.anything());
  });

  it("does not show or post a self-rating after a correct practice-style answer", async () => {
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

    expect(screen.queryByText("Takrorlash: tez javob → oson; sekin → qiyin")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Qiyin" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Yaxshi" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Oson" })).not.toBeInTheDocument();
    expect(post).not.toHaveBeenCalledWith("learn/review", expect.anything());
  });

  it("advances immediately without posting a default self-rating", async () => {
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
    expect(screen.getByText("Ikkinchi savol")).toBeInTheDocument();
    expect(post).not.toHaveBeenCalledWith("learn/review", expect.anything());
  });

  it("keeps the fixed session shell while only the answer list may scroll", () => {
    mockEngine(activeSession());

    renderPage();

    const stage = screen.getByTestId("question-stage");
    const shell = stage.closest(".session-shell");
    const contentCard = stage.closest(".session-content-card");
    const answerList = screen.getByTestId("answer-list");

    expect(shell).toHaveClass("overflow-hidden");
    expect(contentCard).toHaveClass("min-h-0", "overflow-hidden");
    expect(answerList).toHaveClass("session-answer-list", "lg:overflow-y-auto");
    expect(stage.querySelector(".session-question-copy")).not.toHaveClass("overflow-y-auto");
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

    expect(screen.getByText("Tabriklaymiz — o'tdingiz!")).toBeInTheDocument();
    expect(screen.getByRole("dialog", { name: "Tabriklaymiz!" })).toBeInTheDocument();
    expect(screen.getByText("Savollar bo'yicha tahlil")).toBeInTheDocument();
    expect(screen.getAllByText("3.27 belgisi")).toHaveLength(2);
    fireEvent.click(screen.getByRole("button", { name: "Natijani ko'rish" }));
    fireEvent.click(screen.getByText("Rasmiy izohni ochish"));
    expect(screen.getByText("Rasmiy yakuniy izoh")).toBeInTheDocument();
  });

  it("auto-finishes an exam when every question is answered", async () => {
    const finished = activeSession({
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
        }),
      ],
    });
    const finishSession = vi.fn().mockResolvedValue(finished);
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
        total: 1,
      }),
      { finishSession }
    );

    renderPage();

    await waitFor(() => expect(finishSession).toHaveBeenCalledWith("sess-123"));
  });

  it("praises a strong bilet finish without the exam celebration dialog", () => {
    mockEngine(
      activeSession({
        mode: "variant",
        status: "completed",
        score: 18,
        total: 20,
        passed: true,
        completed_at: "2026-07-22T12:00:00Z",
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

    expect(screen.getByText("Zo'r natija!")).toBeInTheDocument();
    expect(screen.queryByRole("dialog", { name: "Tabriklaymiz!" })).not.toBeInTheDocument();
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

// Walks every exit this screen can take for a cookie-less classroom kiosk
// browser (frontend/src/app/[locale]/(kiosk)/station/session/[id]/page.tsx
// reuses this component with kiosk=true) and checks each destination against
// the same PROTECTED_SEGMENTS gate src/proxy.ts enforces: the header exit,
// the completed-screen buttons, the exam-mode close button, the exam-pass
// celebration's dashboard button, and the locale switcher.
describe("SessionPage kiosk mode", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    navigation.push.mockReset();
    navigation.replace.mockReset();
    vi.mocked(useSessionEngine).mockReset();
    vi.mocked(trackEvent).mockReset();
    vi.spyOn(apiClient, "apiGet").mockResolvedValue([] as never);
    vi.spyOn(apiClient, "apiPost").mockResolvedValue({ ok: true } as never);
    vi.spyOn(apiClient, "apiDelete").mockResolvedValue({ ok: true } as never);
  });

  it("sends the header exit button to a kiosk-reachable route", () => {
    mockEngine(activeSession());
    renderKioskPage();

    fireEvent.click(screen.getByRole("button", { name: "Chiqish" }));
    expect(navigation.push).toHaveBeenCalledTimes(1);
    const target = navigation.push.mock.calls[0][0] as string;
    expect(target).toBe("/uz-Latn/station");
    expect(isKioskReachable(target)).toBe(true);
  });

  it("inherits the practice self-rating removal", () => {
    mockEngine(
      activeSession({
        mode: "practice",
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

    renderKioskPage();

    expect(screen.queryByText("Takrorlash: tez javob → oson; sekin → qiyin")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Qiyin" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Yaxshi" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Oson" })).not.toBeInTheDocument();
  });

  it("sends the not-found error card's back button to a kiosk-reachable route", () => {
    mockEngine(null);
    renderKioskPage();

    fireEvent.click(screen.getByRole("button", { name: "Biletlarga qaytish" }));
    const target = navigation.push.mock.calls[0][0] as string;
    expect(target).toBe("/uz-Latn/station/tickets");
    expect(isKioskReachable(target)).toBe(true);
  });

  it("sends both completed-screen buttons to kiosk-reachable routes for a practice session", () => {
    mockEngine(
      activeSession({
        mode: "practice",
        status: "completed",
        score: 15,
        total: 20,
        passed: null,
        completed_at: "2026-07-22T12:00:00Z",
        questions: [],
      })
    );
    renderKioskPage();

    fireEvent.click(screen.getByRole("button", { name: "Mashqqa qaytish" }));
    fireEvent.click(screen.getByRole("button", { name: "Bosh sahifa" }));

    expect(navigation.push).toHaveBeenCalledTimes(2);
    const targets = navigation.push.mock.calls.map((call) => call[0] as string);
    expect(targets).toEqual(["/uz-Latn/station/practice", "/uz-Latn/station"]);
    for (const target of targets) expect(isKioskReachable(target)).toBe(true);
  });

  it("keeps the exam close (X) button under /station instead of /dashboard", () => {
    mockEngine(
      activeSession({
        mode: "exam",
        time_limit_sec: 1500,
        remaining_sec: 300,
        questions: [question()],
      })
    );
    renderKioskPage();

    expect(screen.getByRole("img", { name: "Qaysi belgi to'xtashni taqiqlaydi?" })).toHaveAttribute(
      "src",
      QUESTION_IMAGE_PLACEHOLDER
    );

    fireEvent.click(screen.getByRole("button", { name: "Chiqish" }));
    const dialog = screen.getByRole("dialog");
    fireEvent.click(within(dialog).getByRole("button", { name: "Chiqish" }));
    const target = navigation.push.mock.calls[0][0] as string;
    expect(target).toBe("/uz-Latn/station");
    expect(isKioskReachable(target)).toBe(true);
  });

  it("keeps the exam-pass celebration's dashboard button under /station", () => {
    mockEngine(
      activeSession({
        mode: "exam",
        status: "completed",
        score: 20,
        total: 20,
        passed: true,
        completed_at: "2026-07-22T12:00:00Z",
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
    renderKioskPage();

    const dialog = screen.getByRole("dialog", { name: "Tabriklaymiz!" });
    fireEvent.click(within(dialog).getByRole("button", { name: "Bosh sahifa" }));
    const target = navigation.push.mock.calls[0][0] as string;
    expect(target).toBe("/uz-Latn/station");
    expect(isKioskReachable(target)).toBe(true);
  });

  it("keeps the locale switcher on /station/session/:id, not /session/:id", async () => {
    mockEngine(activeSession());
    renderKioskPage();

    fireEvent.change(screen.getByLabelText("Sessiya tili"), { target: { value: "ru" } });

    await waitFor(() => expect(navigation.replace).toHaveBeenCalledWith("/ru/station/session/sess-123"));
    expect(isKioskReachable(navigation.replace.mock.calls[0][0])).toBe(true);
  });
});
