import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@/lib/api-client";
import * as apiClient from "@/lib/api-client";
import { useSessionEngine } from "./use-session-engine";

const LOCALE = "uz-Latn";
const STARTED_AT = "2026-07-22T10:00:00Z";

function questionDetail(id: string, overrides: Record<string, unknown> = {}) {
  return {
    id,
    category_code: "yhq",
    text: `Savol ${id}`,
    image_url: null,
    answers: [
      { id: `${id}-a1`, position: 1, text: "Birinchi javob", image_url: null },
      { id: `${id}-a2`, position: 2, text: "Ikkinchi javob", image_url: null },
    ],
    signs: [],
    explanation: null,
    position: 1,
    answered: false,
    ...overrides,
  };
}

function startResponse(overrides: Record<string, unknown> = {}) {
  return {
    id: "sess-99",
    mode: "variant",
    question_ids: ["q-1"],
    time_limit_sec: null,
    total: 1,
    started_at: STARTED_AT,
    ...overrides,
  };
}

function scopedQuestionPath(sessionId: string, questionId: string, locale = LOCALE) {
  return `sessions/${sessionId}/questions/${questionId}?locale=${locale}`;
}

function mockOnlyScopedQuestions(
  sessionId = "sess-99",
  details: Record<string, ReturnType<typeof questionDetail>> = {}
) {
  return vi.spyOn(apiClient, "apiGet").mockImplementation(async (path: string) => {
    const match = path.match(/^sessions\/([^/]+)\/questions\/([^?]+)\?locale=(.+)$/);
    if (match && match[1] === sessionId) {
      const id = match[2];
      return (details[id] ?? questionDetail(id)) as never;
    }
    throw new Error(`unexpected apiGet path: ${path}`);
  });
}

async function startOneQuestionSession(
  result: { current: ReturnType<typeof useSessionEngine> },
  mode: "variant" | "exam" = "variant"
) {
  await act(async () => {
    await result.current.startSession(mode, { locale: LOCALE });
  });
}

describe("useSessionEngine", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-07-22T10:05:00Z"));
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("starts with authenticated session-scoped questions in the server's question_ids order", async () => {
    const post = vi.spyOn(apiClient, "apiPost").mockResolvedValue(
      startResponse({
        mode: "exam",
        question_ids: ["q-2", "q-1"],
        time_limit_sec: 1500,
        total: 2,
      }) as never
    );
    const get = mockOnlyScopedQuestions("sess-99", {
      "q-1": questionDetail("q-1", { position: 1 }),
      "q-2": questionDetail("q-2", { position: 2 }),
    });
    const { result } = renderHook(() => useSessionEngine());

    await act(async () => {
      await result.current.startSession("exam", { locale: LOCALE });
    });

    expect(result.current.session?.id).toBe("sess-99");
    expect(result.current.session?.questions.map((question) => question.id)).toEqual(["q-2", "q-1"]);
    expect(result.current.session?.time_limit_sec).toBe(1500);
    expect(result.current.session?.remaining_sec).toBe(1200);
    expect(result.current.session?.questions[0].correct).toBeUndefined();
    expect(result.current.session?.questions[0].correct_answer_id).toBeUndefined();
    expect(post).toHaveBeenCalledWith("sessions", { mode: "exam", locale: LOCALE });
    expect(get.mock.calls.map(([path]) => path)).toEqual([
      scopedQuestionPath("sess-99", "q-2"),
      scopedQuestionPath("sess-99", "q-1"),
    ]);
    expect(get.mock.calls.some(([path]) => String(path).startsWith("questions/"))).toBe(false);
  });

  it("serializes human-readable selectors and count without changing the backend contract", async () => {
    const post = vi.spyOn(apiClient, "apiPost").mockResolvedValue(startResponse() as never);
    mockOnlyScopedQuestions();
    const { result } = renderHook(() => useSessionEngine());

    await act(async () => {
      await result.current.startSession("practice", {
        variant_id: 12,
        category_id: "priority_intersections",
        sign_id: "3.27",
        question_count: 20,
        locale: "ru",
      });
    });

    expect(post).toHaveBeenCalledWith("sessions", {
      mode: "practice",
      locale: "ru",
      variant_id: "12",
      category_id: "priority_intersections",
      sign_id: "3.27",
      count: 20,
    });
    expect(apiClient.apiGet).toHaveBeenCalledWith(scopedQuestionPath("sess-99", "q-1", "ru"));
  });

  it.each([
    [402, "vip_required", "active entitlement required"],
    [429, "daily_limit_reached", "daily practice limit reached"],
  ])("surfaces backend start errors (%s/%s) without fabricating a session", async (status, code, message) => {
    vi.spyOn(apiClient, "apiPost").mockRejectedValue(new ApiError(message, code, status));
    const { result } = renderHook(() => useSessionEngine());

    await act(async () => {
      await result.current.startSession("exam", { locale: LOCALE });
    });

    expect(result.current.session).toBeNull();
    expect(result.current.error).toEqual({ code, message });
  });

  it("surfaces a rejected fetch as network_error", async () => {
    vi.spyOn(apiClient, "apiPost").mockRejectedValue(new TypeError("Failed to fetch"));
    const { result } = renderHook(() => useSessionEngine());

    await act(async () => {
      await result.current.startSession("exam", { locale: LOCALE });
    });

    expect(result.current.session).toBeNull();
    expect(result.current.error?.code).toBe("network_error");
  });

  it("resumes in position order, merges persisted answer fields, and keeps the original exam deadline", async () => {
    vi.setSystemTime(new Date("2026-07-22T10:10:00Z"));
    const realExplanation = {
      legal_refs: [{ document: "YHQ", clause: "3.27" }],
      blocks: [
        { type: "muhim", text: "REAL_TEXT_SENTINEL" },
        {
          type: "answer_analysis",
          items: [{ position: 1, correct: false, text: "REAL_ITEM_SENTINEL" }],
        },
        { type: "tip", content: "LEGACY_CONTENT_FALLBACK" },
      ],
    };
    const get = vi.spyOn(apiClient, "apiGet").mockImplementation(async (path: string) => {
      if (path === "sessions/sess-7") {
        return {
          id: "sess-7",
          mode: "exam",
          total: 2,
          status: "in_progress",
          stopped_reason: "",
          time_limit_sec: 1500,
          started_at: STARTED_AT,
          answers: [
            { question_id: "q-b", position: 2, answered: false },
            {
              question_id: "q-a",
              position: 1,
              answered: true,
              user_answer_id: "q-a-a1",
              correct: false,
              correct_answer_id: "q-a-a2",
            },
          ],
        } as never;
      }
      if (path === scopedQuestionPath("sess-7", "q-a")) {
        return questionDetail("q-a", { position: 1, explanation: realExplanation }) as never;
      }
      if (path === scopedQuestionPath("sess-7", "q-b")) {
        return questionDetail("q-b", { position: 2 }) as never;
      }
      throw new Error(`unexpected apiGet path: ${path}`);
    });
    const { result } = renderHook(() => useSessionEngine());

    await act(async () => {
      await result.current.loadSession("sess-7", LOCALE);
    });

    const questions = result.current.session?.questions ?? [];
    expect(questions.map((question) => question.id)).toEqual(["q-a", "q-b"]);
    expect(questions[0]).toMatchObject({
      answered: true,
      user_answer_id: "q-a-a1",
      correct: false,
      correct_answer_id: "q-a-a2",
    });
    expect(questions[0].explanation?.legal_refs).toEqual(realExplanation.legal_refs);
    expect(questions[0].explanation?.blocks).toEqual([
      { type: "muhim", text: "REAL_TEXT_SENTINEL", content: "REAL_TEXT_SENTINEL" },
      {
        type: "answer_analysis",
        content: "REAL_ITEM_SENTINEL",
        items: [{ position: 1, correct: false, text: "REAL_ITEM_SENTINEL" }],
      },
      { type: "tip", text: "LEGACY_CONTENT_FALLBACK", content: "LEGACY_CONTENT_FALLBACK" },
    ]);
    expect(result.current.session?.remaining_sec).toBe(900);
    expect(result.current.session?.remaining_sec).not.toBe(1500);
    expect(get.mock.calls.map(([path]) => path)).toEqual([
      "sessions/sess-7",
      scopedQuestionPath("sess-7", "q-a"),
      scopedQuestionPath("sess-7", "q-b"),
    ]);
  });

  it("clamps an expired resumed exam to zero instead of restarting its timer", async () => {
    vi.setSystemTime(new Date("2026-07-22T11:00:00Z"));
    vi.spyOn(apiClient, "apiGet").mockImplementation(async (path: string) => {
      if (path === "sessions/sess-expired") {
        return {
          id: "sess-expired",
          mode: "exam",
          total: 1,
          status: "in_progress",
          stopped_reason: "",
          time_limit_sec: 1500,
          started_at: STARTED_AT,
          answers: [{ question_id: "q-1", position: 1, answered: false }],
        } as never;
      }
      if (path === scopedQuestionPath("sess-expired", "q-1")) {
        return questionDetail("q-1") as never;
      }
      throw new Error(`unexpected apiGet path: ${path}`);
    });
    const { result } = renderHook(() => useSessionEngine());

    await act(async () => {
      await result.current.loadSession("sess-expired", LOCALE);
    });

    expect(result.current.session?.time_limit_sec).toBe(1500);
    expect(result.current.session?.remaining_sec).toBe(0);
  });

  it("soft-reloads the same session on locale switch without wiping to loading null", async () => {
    const get = vi.spyOn(apiClient, "apiGet").mockImplementation(async (path: string) => {
      if (path === "sessions/sess-soft") {
        return {
          id: "sess-soft",
          mode: "practice",
          total: 1,
          status: "in_progress",
          stopped_reason: "",
          time_limit_sec: null,
          started_at: STARTED_AT,
          answers: [{ question_id: "q-1", position: 1, answered: false }],
        } as never;
      }
      if (path === scopedQuestionPath("sess-soft", "q-1", "uz-Latn")) {
        return questionDetail("q-1", { text: "Lotin savol" }) as never;
      }
      if (path === scopedQuestionPath("sess-soft", "q-1", "ru")) {
        return questionDetail("q-1", { text: "Русский вопрос" }) as never;
      }
      throw new Error(`unexpected apiGet path: ${path}`);
    });
    const { result } = renderHook(() => useSessionEngine());

    await act(async () => {
      await result.current.loadSession("sess-soft", "uz-Latn");
    });
    expect(result.current.session?.questions[0]?.question).toBe("Lotin savol");
    expect(result.current.loading).toBe(false);

    let midLoadHadSession = false;
    await act(async () => {
      const pending = result.current.loadSession("sess-soft", "ru");
      midLoadHadSession = result.current.session?.id === "sess-soft";
      await pending;
    });

    expect(midLoadHadSession).toBe(true);
    expect(result.current.loading).toBe(false);
    expect(result.current.session?.questions[0]?.question).toBe("Русский вопрос");
    expect(get.mock.calls.filter(([path]) => path === "sessions/sess-soft")).toHaveLength(2);
  });

  it("returns the exact submit DTO and copies backend grading plus real explanation into state", async () => {
    const backendResponse = {
      recorded: true,
      correct: false,
      correct_answer_id: "q-1-a2",
      explanation: {
        legal_refs: ["YHQ 3.27"],
        blocks: [
          { type: "muhim", text: "SUBMIT_REAL_TEXT_SENTINEL" },
          {
            type: "answer_analysis",
            items: [{ position: 1, correct: false, text: "SUBMIT_REAL_ITEM_SENTINEL" }],
          },
        ],
      },
    };
    const post = vi
      .spyOn(apiClient, "apiPost")
      .mockResolvedValueOnce(startResponse() as never)
      .mockResolvedValueOnce(backendResponse as never);
    mockOnlyScopedQuestions();
    const { result } = renderHook(() => useSessionEngine());
    await startOneQuestionSession(result);

    let returned: Awaited<ReturnType<typeof result.current.submitAnswer>> = null;
    await act(async () => {
      returned = await result.current.submitAnswer("sess-99", "q-1", "q-1-a1");
    });

    expect(returned).toBe(backendResponse);
    expect(result.current.session?.questions[0]).toMatchObject({
      user_answer_id: "q-1-a1",
      answered: true,
      correct: false,
      correct_answer_id: "q-1-a2",
    });
    expect(result.current.session?.questions[0].explanation?.blocks[0]).toEqual({
      type: "muhim",
      text: "SUBMIT_REAL_TEXT_SENTINEL",
      content: "SUBMIT_REAL_TEXT_SENTINEL",
    });
    expect(result.current.session?.questions[0].explanation?.blocks[1].items?.[0].text).toBe(
      "SUBMIT_REAL_ITEM_SENTINEL"
    );
    expect(post).toHaveBeenLastCalledWith("sessions/sess-99/answers", {
      question_id: "q-1",
      answer_id: "q-1-a1",
    });
  });

  it("forwards optional FSRS fields on submitAnswer", async () => {
    const post = vi
      .spyOn(apiClient, "apiPost")
      .mockResolvedValueOnce(startResponse() as never)
      .mockResolvedValueOnce({ recorded: true, correct: true, correct_answer_id: "q-1-a1" } as never);
    mockOnlyScopedQuestions();
    const { result } = renderHook(() => useSessionEngine());
    await startOneQuestionSession(result);

    await act(async () => {
      await result.current.submitAnswer("sess-99", "q-1", "q-1-a1", {
        latencyMs: 1200,
        skipFsrs: true,
        rating: 4,
      });
    });

    expect(post).toHaveBeenLastCalledWith("sessions/sess-99/answers", {
      question_id: "q-1",
      answer_id: "q-1-a1",
      rating: 4,
      latency_ms: 1200,
      skip_fsrs: true,
    });
  });

  it("records exam answer grades from the backend (never invents them client-side)", async () => {
    vi.spyOn(apiClient, "apiPost")
      .mockResolvedValueOnce(startResponse({ mode: "exam", time_limit_sec: 1500 }) as never)
      .mockResolvedValueOnce({
        recorded: true,
        stopped: false,
        correct: true,
        correct_answer_id: "q-1-a1",
      } as never);
    const get = mockOnlyScopedQuestions();
    const { result } = renderHook(() => useSessionEngine());
    await startOneQuestionSession(result, "exam");

    await act(async () => {
      await result.current.submitAnswer("sess-99", "q-1", "q-1-a1");
    });

    expect(result.current.session?.questions[0].user_answer_id).toBe("q-1-a1");
    expect(result.current.session?.questions[0].correct).toBe(true);
    expect(result.current.session?.questions[0].correct_answer_id).toBe("q-1-a1");
    expect(get).toHaveBeenCalledTimes(1);
  });

  it("does not mutate local answer state when submit fails", async () => {
    vi.spyOn(apiClient, "apiPost")
      .mockResolvedValueOnce(startResponse() as never)
      .mockRejectedValueOnce(new TypeError("Failed to fetch"));
    mockOnlyScopedQuestions();
    const { result } = renderHook(() => useSessionEngine());
    await startOneQuestionSession(result);

    await act(async () => {
      await result.current.submitAnswer("sess-99", "q-1", "q-1-a1");
    });

    expect(result.current.session?.questions[0].user_answer_id).toBeNull();
    expect(result.current.session?.questions[0].correct).toBeUndefined();
    expect(result.current.error?.code).toBe("network_error");
  });

  it("reloads a server-stopped exam so completed feedback is immediately available", async () => {
    const stoppedResponse = {
      recorded: true,
      stopped: true,
      stop_reason: "too_many_errors",
    };
    let stopped = false;
    vi.spyOn(apiClient, "apiPost").mockImplementation(async (path: string) => {
      if (path === "sessions") {
        return startResponse({ mode: "exam", time_limit_sec: 1500 }) as never;
      }
      if (path === "sessions/sess-99/answers") {
        stopped = true;
        return stoppedResponse as never;
      }
      throw new Error(`unexpected apiPost path: ${path}`);
    });
    const get = vi.spyOn(apiClient, "apiGet").mockImplementation(async (path: string) => {
      if (path === "sessions/sess-99") {
        return {
          id: "sess-99",
          mode: "exam",
          total: 1,
          status: "failed",
          stopped_reason: "too_many_errors",
          score: 0,
          time_limit_sec: 1500,
          started_at: STARTED_AT,
          finished_at: "2026-07-22T10:05:00Z",
          answers: [
            {
              question_id: "q-1",
              position: 1,
              answered: true,
              user_answer_id: "q-1-a1",
              correct: false,
              correct_answer_id: "q-1-a2",
            },
          ],
        } as never;
      }
      if (path === scopedQuestionPath("sess-99", "q-1")) {
        return questionDetail("q-1", {
          answered: stopped,
          user_answer_id: stopped ? "q-1-a1" : undefined,
          correct: stopped ? false : undefined,
          correct_answer_id: stopped ? "q-1-a2" : undefined,
          explanation: stopped
            ? {
                legal_refs: [],
                blocks: [{ type: "muhim", text: "STOPPED_FEEDBACK_SENTINEL" }],
              }
            : null,
        }) as never;
      }
      throw new Error(`unexpected apiGet path: ${path}`);
    });
    const { result } = renderHook(() => useSessionEngine());
    await startOneQuestionSession(result, "exam");

    let returned: Awaited<ReturnType<typeof result.current.submitAnswer>> = null;
    await act(async () => {
      returned = await result.current.submitAnswer("sess-99", "q-1", "q-1-a1");
    });

    expect(returned).toBe(stoppedResponse);
    expect(result.current.session?.status).toBe("completed");
    expect(result.current.session?.stopped_reason).toBe("too_many_errors");
    expect(result.current.session?.questions[0].correct).toBe(false);
    expect(result.current.session?.questions[0].correct_answer_id).toBe("q-1-a2");
    expect(result.current.session?.questions[0].explanation?.blocks[0].content).toBe(
      "STOPPED_FEEDBACK_SENTINEL"
    );
    expect(get.mock.calls.map(([path]) => path)).toEqual([
      scopedQuestionPath("sess-99", "q-1"),
      "sessions/sess-99",
      scopedQuestionPath("sess-99", "q-1"),
    ]);
  });

  it("finishSession reloads detail and scoped questions to disclose completed exam feedback", async () => {
    let finished = false;
    vi.spyOn(apiClient, "apiPost").mockImplementation(async (path: string) => {
      if (path === "sessions") {
        return startResponse({ mode: "exam", time_limit_sec: 1500 }) as never;
      }
      if (path === "sessions/sess-99/finish") {
        finished = true;
        return {
          status: "passed",
          stopped_reason: "completed",
          score: 1,
          total: 1,
        } as never;
      }
      throw new Error(`unexpected apiPost path: ${path}`);
    });
    const get = vi.spyOn(apiClient, "apiGet").mockImplementation(async (path: string) => {
      if (path === "sessions/sess-99") {
        return {
          id: "sess-99",
          mode: "exam",
          total: 1,
          status: "passed",
          stopped_reason: "completed",
          score: 1,
          time_limit_sec: 1500,
          started_at: STARTED_AT,
          finished_at: "2026-07-22T10:05:00Z",
          answers: [
            {
              question_id: "q-1",
              position: 1,
              answered: true,
              user_answer_id: "q-1-a1",
              correct: true,
              correct_answer_id: "q-1-a1",
            },
          ],
        } as never;
      }
      if (path === scopedQuestionPath("sess-99", "q-1")) {
        return questionDetail("q-1", {
          answered: finished,
          user_answer_id: finished ? "q-1-a1" : undefined,
          correct: finished ? true : undefined,
          correct_answer_id: finished ? "q-1-a1" : undefined,
          explanation: finished
            ? {
                legal_refs: [],
                blocks: [{ type: "muhim", text: "FINISH_FEEDBACK_SENTINEL" }],
              }
            : null,
        }) as never;
      }
      throw new Error(`unexpected apiGet path: ${path}`);
    });
    const { result } = renderHook(() => useSessionEngine());
    await startOneQuestionSession(result, "exam");

    await act(async () => {
      await result.current.finishSession("sess-99");
    });

    expect(result.current.session).toMatchObject({
      status: "completed",
      passed: true,
      score: 1,
      total: 1,
      stopped_reason: "completed",
      completed_at: "2026-07-22T10:05:00Z",
    });
    expect(result.current.session?.questions[0]).toMatchObject({
      user_answer_id: "q-1-a1",
      correct: true,
      correct_answer_id: "q-1-a1",
    });
    expect(result.current.session?.questions[0].explanation?.blocks[0].content).toBe(
      "FINISH_FEEDBACK_SENTINEL"
    );
    expect(get.mock.calls.map(([path]) => path)).toEqual([
      scopedQuestionPath("sess-99", "q-1"),
      "sessions/sess-99",
      scopedQuestionPath("sess-99", "q-1"),
    ]);
  });

  it("does not mark a session completed when the finish POST fails", async () => {
    vi.spyOn(apiClient, "apiPost")
      .mockResolvedValueOnce(startResponse() as never)
      .mockRejectedValueOnce(new ApiError("boom", "internal", 500));
    mockOnlyScopedQuestions();
    const { result } = renderHook(() => useSessionEngine());
    await startOneQuestionSession(result);

    await act(async () => {
      await result.current.finishSession("sess-99");
    });

    expect(result.current.session?.status).toBe("active");
    expect(result.current.error?.code).toBe("internal");
  });
});
