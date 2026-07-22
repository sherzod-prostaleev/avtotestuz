import { renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import * as apiClient from "@/lib/api-client";
import { useSessionHistory } from "./use-session-history";

describe("useSessionHistory", () => {
  beforeEach(() => vi.restoreAllMocks());

  it("loads the real session-summary contract with a bounded limit", async () => {
    vi.spyOn(apiClient, "apiGet").mockResolvedValue([
      {
        id: "session-1",
        mode: "exam",
        status: "in_progress",
        total: 20,
        started_at: "2026-07-22T10:00:00Z",
      },
    ]);

    const { result } = renderHook(() => useSessionHistory(10));
    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(apiClient.apiGet).toHaveBeenCalledWith("me/sessions?limit=10");
    expect(result.current.sessions[0]).toMatchObject({ id: "session-1", status: "in_progress" });
  });
});
