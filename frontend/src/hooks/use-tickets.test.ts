import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import { useTickets } from "./use-tickets";
import * as apiClient from "@/lib/api-client";

describe("useTickets", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("fetches tickets array successfully", async () => {
    const mockTickets = [
      { id: 1, number: 1, total_questions: 20, status: "completed", score: 19, passed: true },
      { id: 2, number: 2, total_questions: 20, status: "in_progress", score: null, passed: null },
      { id: 3, number: 3, total_questions: 20, status: "locked", score: null, passed: null },
    ];

    vi.spyOn(apiClient, "apiGet").mockResolvedValue(mockTickets as any);

    const { result } = renderHook(() => useTickets());

    expect(result.current.loading).toBe(true);

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.tickets.length).toBe(3);
    expect(result.current.tickets[0].score).toBe(19);
    expect(result.current.tickets[2].status).toBe("locked");
  });
});
