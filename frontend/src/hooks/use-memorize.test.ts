import { renderHook, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import * as apiClient from "@/lib/api-client";
import { useMemorize } from "./use-memorize";

const sampleDetail = {
  id: "q-1",
  category_code: "signs",
  text: "Savol matni?",
  image_url: null,
  answers: [
    { id: "a-1", position: 1, text: "Variant A", image_url: null },
    { id: "a-2", position: 2, text: "Variant B", image_url: null },
  ],
  signs: [],
  explanation: null,
  position: 1,
  answered: true,
  correct_answer_id: "a-2",
};

describe("useMemorize", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("fetches the topic and maps each item into a session question item", async () => {
    vi.spyOn(apiClient, "apiGet").mockResolvedValue([sampleDetail]);

    const { result } = renderHook(() => useMemorize("signs", "uz-Latn"));
    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(apiClient.apiGet).toHaveBeenCalledWith("categories/signs/memorize?locale=uz-Latn");
    expect(result.current.error).toBeNull();
    expect(result.current.questions).toHaveLength(1);
    expect(result.current.questions[0].question).toBe("Savol matni?");
    expect(result.current.questions[0].correct_answer_id).toBe("a-2");
  });

  it("surfaces the server error code instead of the raw questions", async () => {
    vi.spyOn(apiClient, "apiGet").mockRejectedValue(
      new apiClient.ApiError("active entitlement required", "vip_required", 402)
    );

    const { result } = renderHook(() => useMemorize("signs", "uz-Latn"));
    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(result.current.error).toEqual({
      code: "vip_required",
      message: "active entitlement required",
    });
    expect(result.current.questions).toEqual([]);
  });
});
