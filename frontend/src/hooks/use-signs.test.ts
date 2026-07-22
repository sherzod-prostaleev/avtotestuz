import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import { useSigns } from "./use-signs";
import * as apiClient from "@/lib/api-client";

describe("useSigns", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("fetches signs list successfully", async () => {
    const mockSigns = [
      {
        id: "s-1",
        code: "3.27",
        group_code: "prohibitory",
        group_name: "Taqiqlovchi",
        name: "To'xtash taqiqlangan",
        description: "Transport vositalarining to'xtashi taqiqlanadi",
        image_url: "/signs/3.27.png",
      },
    ];

    vi.spyOn(apiClient, "apiGet").mockResolvedValue(mockSigns as any);

    const { result } = renderHook(() => useSigns("prohibitory", "to'xtash"));

    expect(result.current.loading).toBe(true);

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.signs.length).toBe(1);
    expect(result.current.signs[0].code).toBe("3.27");
    expect(apiClient.apiGet).toHaveBeenCalledWith("signs?group=prohibitory&q=to%27xtash");
  });
});
