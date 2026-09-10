import { describe, it, expect, vi, beforeEach } from "vitest";
import { act, renderHook, waitFor } from "@testing-library/react";
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

  it("clears bilet progress and re-reads the grid from the server", async () => {
    const cleared = { number: 1, question_count: 20, unlocked: true, best_correct: 0, attempts: 0 };
    const played = { ...cleared, best_correct: 19, attempts: 2, completed_at: "2026-07-20T12:00:00Z" };

    const apiGet = vi
      .spyOn(apiClient, "apiGet")
      .mockResolvedValueOnce([played] as any)
      .mockResolvedValueOnce([cleared] as any);
    const apiPost = vi
      .spyOn(apiClient, "apiPost")
      .mockResolvedValue({ cleared: 1, unlock_ceiling: 2 } as any);

    const { result } = renderHook(() => useTickets());
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.tickets[0].status).toBe("completed");

    let ok: boolean | undefined;
    await act(async () => {
      ok = await result.current.clearProgress();
    });

    expect(ok).toBe(true);
    expect(apiPost).toHaveBeenCalledWith("me/variants/reset");
    // The grid comes back from the server, not from a local guess: only the
    // server knows which bilets stayed open.
    expect(apiGet).toHaveBeenCalledTimes(2);
    expect(result.current.tickets[0].status).toBe("unstarted");
    expect(result.current.clearedCount).toBe(1);
    expect(result.current.clearError).toBeNull();
    expect(result.current.clearing).toBe(false);
  });

  it("reports a failed clear and leaves the grid untouched", async () => {
    const played = {
      number: 1,
      question_count: 20,
      unlocked: true,
      best_correct: 19,
      attempts: 2,
      completed_at: "2026-07-20T12:00:00Z",
    };
    const apiGet = vi.spyOn(apiClient, "apiGet").mockResolvedValue([played] as any);
    vi.spyOn(apiClient, "apiPost").mockRejectedValue(
      new apiClient.ApiError("reset failed", "internal", 500)
    );

    const { result } = renderHook(() => useTickets());
    await waitFor(() => expect(result.current.loading).toBe(false));

    let ok: boolean | undefined;
    await act(async () => {
      ok = await result.current.clearProgress();
    });

    expect(ok).toBe(false);
    expect(result.current.clearError).toBe("reset failed");
    expect(result.current.clearedCount).toBeNull();
    expect(result.current.clearing).toBe(false);
    // No refetch on the failure path — the grid on screen is still correct.
    expect(apiGet).toHaveBeenCalledTimes(1);
    expect(result.current.tickets[0].status).toBe("completed");

    act(() => result.current.dismissClearNotice());
    expect(result.current.clearError).toBeNull();
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
