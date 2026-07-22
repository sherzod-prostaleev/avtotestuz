import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useSessionEngine } from "./use-session-engine";
import * as apiClient from "@/lib/api-client";
import { ApiError } from "@/lib/api-client";

const LOCALE = "uz-Latn";

/** A minimal QuestionDetail as the real content API returns it (no correctness fields). */
function questionDetail(id: string, text: string) {
  return {
    id,
    category_code: "yhq",
    text,
    image_url: null,
    answers: [
      { id: `${id}-a1`, position: 1, text: "Birinchi javob", image_url: null },
      { id: `${id}-a2`, position: 2, text: "Ikkinchi javob", image_url: null },
    ],
    signs: [],
    explanation: null,
  };
}

/** Configure apiGet to resolve question content by path. */
function mockQuestionContent() {
  vi.spyOn(apiClient, "apiGet").mockImplementation(async (path: string) => {
    const m = path.match(/^questions\/([^?]+)/);
    if (m) return questionDetail(m[1], `Savol ${m[1]}`) as never;
    throw new Error(`unexpected apiGet path: ${path}`);
  });
}

/** Start a single-question session so submit/finish tests have real state. */
async function startOneQuestionSession(result: { current: ReturnType<typeof useSessionEngine> }) {
  vi.spyOn(apiClient, "apiPost").mockResolvedValueOnce({
    id: "sess-99",
    mode: "variant",
    question_ids: ["q-1"],
    time_limit_sec: null,
    total: 1,
    started_at: "2026-07-22T10:00:00Z",
  } as never);
  mockQuestionContent();
  await act(async () => {
    await result.current.startSession("variant", { locale: LOCALE });
  });
}

describe("useSessionEngine", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("starts a session by creating it then fetching each question's content", async () => {
    vi.spyOn(apiClient, "apiPost").mockResolvedValue({
      id: "sess-99",
      mode: "exam",
      question_ids: ["q-1"],
      time_limit_sec: 1500,
      total: 1,
      started_at: "2026-07-22T10:00:00Z",
    } as never);
    mockQuestionContent();

    const { result } = renderHook(() => useSessionEngine());

    let started: Awaited<ReturnType<typeof result.current.startSession>>;
    await act(async () => {
      started = await result.current.startSession("exam", { locale: LOCALE });
    });

    expect(started?.id).toBe("sess-99");
    expect(result.current.session?.mode).toBe("exam");
    expect(result.current.session?.time_limit_sec).toBe(1500);
    expect(result.current.session?.remaining_sec).toBe(1500);
    expect(result.current.session?.questions).toHaveLength(1);
    expect(result.current.session?.questions[0].question).toBe("Savol q-1");
    expect(result.current.session?.questions[0].answers).toHaveLength(2);
    // no correctness leaked from content
    expect(result.current.session?.questions[0].correct).toBeUndefined();
    expect(result.current.session?.questions[0].correct_answer_id).toBeUndefined();
    expect(apiClient.apiPost).toHaveBeenCalledWith("sessions", { mode: "exam", locale: LOCALE });
    expect(apiClient.apiGet).toHaveBeenCalledWith("questions/q-1?locale=uz-Latn");
  });

  it("submitAnswer stores backend correct/correct_answer_id verbatim (variant feedback)", async () => {
    const { result } = renderHook(() => useSessionEngine());
    await startOneQuestionSession(result);

    vi.spyOn(apiClient, "apiPost").mockResolvedValueOnce({
      recorded: true,
      correct: true,
      correct_answer_id: "q-1-a1",
    } as never);

    await act(async () => {
      await result.current.submitAnswer("sess-99", "q-1", "q-1-a1");
    });

    expect(result.current.session?.questions[0].user_answer_id).toBe("q-1-a1");
    expect(result.current.session?.questions[0].correct).toBe(true);
    expect(result.current.session?.questions[0].correct_answer_id).toBe("q-1-a1");
    expect(apiClient.apiPost).toHaveBeenCalledWith("sessions/sess-99/answers", {
      question_id: "q-1",
      answer_id: "q-1-a1",
    });
  });

  it("startSession on 402 vip_required leaves session null and surfaces the code", async () => {
    vi.spyOn(apiClient, "apiPost").mockRejectedValue(
      new ApiError("active entitlement required", "vip_required", 402)
    );
    const { result } = renderHook(() => useSessionEngine());

    await act(async () => {
      await result.current.startSession("exam", { locale: LOCALE });
    });

    expect(result.current.session).toBeNull();
    expect(result.current.error?.code).toBe("vip_required");
  });

  it("startSession on 429 daily_limit_reached leaves session null and surfaces the code", async () => {
    vi.spyOn(apiClient, "apiPost").mockRejectedValue(
      new ApiError("daily practice limit reached", "daily_limit_reached", 429)
    );
    const { result } = renderHook(() => useSessionEngine());

    await act(async () => {
      await result.current.startSession("practice", { locale: LOCALE });
    });

    expect(result.current.session).toBeNull();
    expect(result.current.error?.code).toBe("daily_limit_reached");
  });

  it("startSession on a thrown network error surfaces network_error", async () => {
    vi.spyOn(apiClient, "apiPost").mockRejectedValue(new TypeError("Failed to fetch"));
    const { result } = renderHook(() => useSessionEngine());

    await act(async () => {
      await result.current.startSession("exam", { locale: LOCALE });
    });

    expect(result.current.session).toBeNull();
    expect(result.current.error?.code).toBe("network_error");
  });

  it("submitAnswer in exam-mode-in-progress keeps correct undefined (anti-cheat)", async () => {
    const { result } = renderHook(() => useSessionEngine());
    await startOneQuestionSession(result);

    // Exam-in-progress: backend omits correct / correct_answer_id entirely.
    vi.spyOn(apiClient, "apiPost").mockResolvedValueOnce({
      recorded: true,
      stopped: false,
    } as never);

    await act(async () => {
      await result.current.submitAnswer("sess-99", "q-1", "q-1-a1");
    });

    expect(result.current.session?.questions[0].user_answer_id).toBe("q-1-a1");
    expect(result.current.session?.questions[0].correct).toBeUndefined();
    expect(result.current.session?.questions[0].correct_answer_id).toBeUndefined();
  });

  it("submitAnswer surfaces stopped/stop_reason so the page can react", async () => {
    const { result } = renderHook(() => useSessionEngine());
    await startOneQuestionSession(result);

    vi.spyOn(apiClient, "apiPost").mockResolvedValueOnce({
      recorded: true,
      stopped: true,
      stop_reason: "too_many_errors",
    } as never);

    let resp: Awaited<ReturnType<typeof result.current.submitAnswer>>;
    await act(async () => {
      resp = await result.current.submitAnswer("sess-99", "q-1", "q-1-a1");
    });

    expect(resp?.stopped).toBe(true);
    expect(resp?.stop_reason).toBe("too_many_errors");
  });

  it("submitAnswer on a network error does NOT mutate local answer state", async () => {
    const { result } = renderHook(() => useSessionEngine());
    await startOneQuestionSession(result);

    vi.spyOn(apiClient, "apiPost").mockRejectedValueOnce(new TypeError("Failed to fetch"));

    let resp: Awaited<ReturnType<typeof result.current.submitAnswer>>;
    await act(async () => {
      resp = await result.current.submitAnswer("sess-99", "q-1", "q-1-a1");
    });

    expect(resp).toBeNull();
    // No client-computed correctness, no answer recorded locally.
    expect(result.current.session?.questions[0].user_answer_id ?? null).toBeNull();
    expect(result.current.session?.questions[0].correct).toBeUndefined();
    expect(result.current.session?.questions[0].correct_answer_id).toBeUndefined();
    expect(result.current.error?.code).toBe("network_error");
  });

  it("finishSession stores the real backend result", async () => {
    const { result } = renderHook(() => useSessionEngine());
    await startOneQuestionSession(result);

    vi.spyOn(apiClient, "apiPost").mockResolvedValueOnce({
      status: "passed",
      stopped_reason: "completed",
      score: 18,
      total: 20,
    } as never);

    await act(async () => {
      await result.current.finishSession("sess-99");
    });

    expect(result.current.session?.status).toBe("completed");
    expect(result.current.session?.passed).toBe(true);
    expect(result.current.session?.score).toBe(18);
    expect(result.current.session?.total).toBe(20);
  });

  it("finishSession error does NOT silently mark the session completed", async () => {
    const { result } = renderHook(() => useSessionEngine());
    await startOneQuestionSession(result);

    vi.spyOn(apiClient, "apiPost").mockRejectedValueOnce(
      new ApiError("boom", "internal", 500)
    );

    await act(async () => {
      await result.current.finishSession("sess-99");
    });

    expect(result.current.session?.status).toBe("active");
    expect(result.current.error?.code).toBe("internal");
  });

  it("loadSession reconstructs question order from answers[].position", async () => {
    vi.spyOn(apiClient, "apiGet").mockImplementation(async (path: string) => {
      if (path.startsWith("sessions/")) {
        return {
          id: "sess-7",
          mode: "variant",
          total: 2,
          status: "in_progress",
          stopped_reason: "",
          started_at: "2026-07-22T10:00:00Z",
          answers: [
            { question_id: "q-b", position: 2, answered: false },
            { question_id: "q-a", position: 1, answered: true, correct: true },
          ],
        } as never;
      }
      const m = path.match(/^questions\/([^?]+)/);
      if (m) return questionDetail(m[1], `Savol ${m[1]}`) as never;
      throw new Error(`unexpected apiGet path: ${path}`);
    });

    const { result } = renderHook(() => useSessionEngine());

    await act(async () => {
      await result.current.loadSession("sess-7", LOCALE);
    });

    const qs = result.current.session?.questions ?? [];
    expect(qs.map((q) => q.id)).toEqual(["q-a", "q-b"]);
    expect(qs[0].answered).toBe(true);
    expect(qs[0].correct).toBe(true);
    expect(qs[1].answered).toBe(false);
  });
});
