import { renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import * as apiClient from "@/lib/api-client";
import { useVariantCount } from "./use-variant-count";

vi.mock("@/lib/api-client", async () => {
  const actual = await vi.importActual<typeof import("@/lib/api-client")>("@/lib/api-client");
  return {
    ...actual,
    apiGet: vi.fn(),
  };
});

describe("useVariantCount", () => {
  beforeEach(() => {
    vi.mocked(apiClient.apiGet).mockReset();
  });

  it("returns the live variants list length", async () => {
    vi.mocked(apiClient.apiGet).mockResolvedValue(
      Array.from({ length: 63 }, (_, i) => ({ number: i + 1, question_count: 20 }))
    );

    const { result } = renderHook(() => useVariantCount());

    await waitFor(() => {
      expect(result.current).toBe(63);
    });
    expect(apiClient.apiGet).toHaveBeenCalledWith("variants");
  });

  it("keeps the static fallback when the request fails", async () => {
    vi.mocked(apiClient.apiGet).mockRejectedValue(new Error("offline"));

    const { result } = renderHook(() => useVariantCount());

    await waitFor(() => {
      expect(apiClient.apiGet).toHaveBeenCalled();
    });
    expect(result.current).toBe(63);
  });
});
