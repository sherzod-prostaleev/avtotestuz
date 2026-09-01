import { renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import * as apiClient from "@/lib/api-client";
import {
  OFFICIAL_QUESTION_COUNT,
  OFFICIAL_TICKET_COUNT,
  OFFICIAL_TOPIC_COUNT,
} from "@/lib/content-counts";
import { useCatalogCounts, useVariantCount, clearCatalogCountsCacheForTests } from "./use-variant-count";

vi.mock("@/lib/api-client", async () => {
  const actual = await vi.importActual<typeof import("@/lib/api-client")>("@/lib/api-client");
  return {
    ...actual,
    apiGet: vi.fn(),
  };
});

// Deliberately unlike the static fallback, so a test can only pass by reading
// the catalog response rather than the constant.
const LIVE_TICKETS = 70;

/** The hook asks two endpoints; answer each by path rather than by call order. */
function mockCatalog({
  variants = [],
  categories = [],
}: {
  variants?: Array<{ number: number; question_count?: number }>;
  categories?: Array<{ code: string }>;
}) {
  vi.mocked(apiClient.apiGet).mockImplementation(async (path: string) =>
    (path.startsWith("categories") ? categories : variants) as never
  );
}

describe("useVariantCount", () => {
  beforeEach(() => {
    clearCatalogCountsCacheForTests();
    vi.mocked(apiClient.apiGet).mockReset();
  });

  it("returns the live variants list length", async () => {
    mockCatalog({
      variants: Array.from({ length: LIVE_TICKETS }, (_, i) => ({
        number: i + 1,
        question_count: 20,
      })),
    });

    const { result } = renderHook(() => useVariantCount());

    await waitFor(() => {
      expect(result.current).toBe(LIVE_TICKETS);
    });
    expect(apiClient.apiGet).toHaveBeenCalledWith("variants");
  });

  it("keeps the static fallback when the request fails", async () => {
    vi.mocked(apiClient.apiGet).mockRejectedValue(new Error("offline"));

    const { result } = renderHook(() => useVariantCount());

    await waitFor(() => {
      expect(apiClient.apiGet).toHaveBeenCalled();
    });
    expect(result.current).toBe(OFFICIAL_TICKET_COUNT);
  });
});

describe("useCatalogCounts", () => {
  beforeEach(() => {
    clearCatalogCountsCacheForTests();
    vi.mocked(apiClient.apiGet).mockReset();
  });

  it("sums question_count across the catalog, including a short final bilet", async () => {
    mockCatalog({
      variants: [
        { number: 1, question_count: 20 },
        { number: 2, question_count: 20 },
        { number: 3, question_count: 5 },
      ],
      categories: [{ code: "a" }, { code: "b" }],
    });

    const { result } = renderHook(() => useCatalogCounts());

    await waitFor(() => {
      expect(result.current).toEqual({ tickets: 3, questions: 45, topics: 2 });
    });
  });

  it("falls back to the static question count when the catalog omits counts", async () => {
    mockCatalog({ variants: [{ number: 1 }, { number: 2 }], categories: [{ code: "a" }] });

    const { result } = renderHook(() => useCatalogCounts());

    await waitFor(() => {
      expect(result.current).toEqual({
        tickets: 2,
        questions: OFFICIAL_QUESTION_COUNT,
        topics: 1,
      });
    });
  });

  it("keeps both fallbacks when the request fails", async () => {
    vi.mocked(apiClient.apiGet).mockRejectedValue(new Error("offline"));

    const { result } = renderHook(() => useCatalogCounts());

    await waitFor(() => {
      expect(apiClient.apiGet).toHaveBeenCalled();
    });
    expect(result.current).toEqual({
      tickets: OFFICIAL_TICKET_COUNT,
      questions: OFFICIAL_QUESTION_COUNT,
      topics: OFFICIAL_TOPIC_COUNT,
    });
  });
});
