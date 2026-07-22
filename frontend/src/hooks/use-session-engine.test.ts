import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useSessionEngine } from "./use-session-engine";
import * as apiClient from "@/lib/api-client";

describe("useSessionEngine", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("starts session successfully", async () => {
    const mockSession = {
      id: "sess-99",
      mode: "exam",
      time_limit_sec: 1500,
      remaining_sec: 1500,
      status: "active",
      questions: [
        {
          id: "q-1",
          question: "Chorrahada kim ustun?",
          answers: [
            { id: "a-1", text: "Piyoda" },
            { id: "a-2", text: "Avtomobil" },
          ],
        },
      ],
      score: null,
      passed: null,
      completed_at: null,
    };

    vi.spyOn(apiClient, "apiPost").mockResolvedValue(mockSession as any);

    const { result } = renderHook(() => useSessionEngine());

    let started: any;
    await act(async () => {
      started = await result.current.startSession("exam");
    });

    expect(started?.id).toBe("sess-99");
    expect(result.current.session?.mode).toBe("exam");
    expect(apiClient.apiPost).toHaveBeenCalledWith("sessions", { mode: "exam" });
  });

  it("submits answer and updates session state", async () => {
    const mockAnsweredSession = {
      id: "sess-99",
      mode: "variant",
      status: "active",
      questions: [
        {
          id: "q-1",
          question: "Chorrahada kim ustun?",
          answers: [
            { id: "a-1", text: "Piyoda" },
            { id: "a-2", text: "Avtomobil" },
          ],
          user_answer_id: "a-1",
          is_correct: true,
        },
      ],
    };

    vi.spyOn(apiClient, "apiPost").mockResolvedValue(mockAnsweredSession as any);

    const { result } = renderHook(() => useSessionEngine());

    await act(async () => {
      await result.current.submitAnswer("sess-99", "q-1", "a-1");
    });

    expect(result.current.session?.questions[0].user_answer_id).toBe("a-1");
    expect(apiClient.apiPost).toHaveBeenCalledWith("sessions/sess-99/answers", {
      question_id: "q-1",
      answer_id: "a-1",
    });
  });
});
