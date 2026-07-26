import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import { useTickets } from "./use-tickets";
import * as apiClient from "@/lib/api-client";

describe("useTickets", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("maps real variant status DTOs into ticket items", async () => {
    const mockVariants = [
      {
        number: 1,
        question_count: 20,
        unlocked: true,
        best_correct: 19,
        attempts: 2,
        completed_at: "2026-07-20T12:00:00Z",
      },
      { number: 2, question_count: 20, unlocked: true, best_correct: 12, attempts: 1 },
      {
        number: 3,
        question_count: 20,
        unlocked: false,
        lock_reason: "prev_required",
        best_correct: 0,
        attempts: 0,
      },
      { number: 4, question_count: 20, unlocked: true, best_correct: 0, attempts: 0 },
    ];

    vi.spyOn(apiClient, "apiGet").mockResolvedValue(mockVariants as any);

    const { result } = renderHook(() => useTickets());

    expect(result.current.loading).toBe(true);

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.tickets).toHaveLength(4);
    expect(result.current.tickets[0]).toEqual({
      number: 1,
      total_questions: 20,
      status: "completed",
      best_correct: 19,
      attempts: 2,
      unlocked: true,
      completed_at: "2026-07-20T12:00:00Z",
    });
    expect(result.current.tickets[1].status).toBe("in_progress");
    expect(result.current.tickets[2].status).toBe("locked");
    expect(result.current.tickets[2].lock_reason).toBe("prev_required");
    expect(result.current.tickets[3].status).toBe("unstarted");
    expect(apiClient.apiGet).toHaveBeenCalledWith("me/variants");
  });

  it("exposes API failures", async () => {
    vi.spyOn(apiClient, "apiGet").mockRejectedValue(
      new apiClient.ApiError("variants unavailable", "internal", 500)
    );

    const { result } = renderHook(() => useTickets());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.error).toBe("variants unavailable");
    expect(result.current.tickets).toEqual([]);
  });
});
