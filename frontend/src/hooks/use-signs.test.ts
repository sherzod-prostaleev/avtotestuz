import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import * as apiClient from "@/lib/api-client";
import { useSignDetail, useSigns } from "./use-signs";

const sign = {
  code: "3.27",
  group_code: "prohibiting",
  name: "To'xtash taqiqlangan",
  image_url: "/signs/3.27.png",
  question_count: 4,
};

describe("useSigns", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("fetches the locale-aware, server-filtered signs list", async () => {
    vi.spyOn(apiClient, "apiGet").mockResolvedValue([sign]);

    const { result } = renderHook(() =>
      useSigns("uz-Cyrl", "prohibiting", " 3.27 ")
    );

    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(result.current.error).toBeNull();
    expect(result.current.signs).toEqual([sign]);
    expect(apiClient.apiGet).toHaveBeenCalledWith(
      "signs?locale=uz-Cyrl&group=prohibiting&q=3.27"
    );
  });

  it("preserves a genuinely empty server dataset instead of substituting local data", async () => {
    vi.spyOn(apiClient, "apiGet").mockResolvedValue([]);

    const { result } = renderHook(() => useSigns("uz-Latn", "all", ""));

    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(result.current.signs).toEqual([]);
    expect(result.current.error).toBeNull();
    expect(apiClient.apiGet).toHaveBeenCalledWith("signs?locale=uz-Latn");
  });

  it("reports API failures without exposing backend text or stale signs", async () => {
    const get = vi
      .spyOn(apiClient, "apiGet")
      .mockResolvedValueOnce([sign])
      .mockRejectedValueOnce(new Error("database address and secret"));

    const { result } = renderHook(() => useSigns("ru"));
    await waitFor(() => expect(result.current.signs).toEqual([sign]));

    await act(async () => {
      await result.current.refetch();
    });

    expect(get).toHaveBeenCalledTimes(2);
    expect(result.current.signs).toEqual([]);
    expect(result.current.error).toBe("unavailable");
    expect(result.current.loading).toBe(false);
  });

  it("rejects malformed list payloads as unavailable", async () => {
    vi.spyOn(apiClient, "apiGet").mockResolvedValue([
      { code: "3.27", name: "Missing API fields" },
    ]);

    const { result } = renderHook(() => useSigns("uz-Latn"));
    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(result.current.signs).toEqual([]);
    expect(result.current.error).toBe("unavailable");
  });
});

describe("useSignDetail", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("loads the verified description from the detail endpoint", async () => {
    const detail = {
      code: "3.27",
      group_code: "prohibiting",
      name: "To'xtash taqiqlangan",
      description: "Transport vositalarining to'xtashi taqiqlanadi.",
      image_url: "/signs/3.27.png",
      question_ids: ["q-1"],
    };
    vi.spyOn(apiClient, "apiGet").mockResolvedValue(detail);

    const { result } = renderHook(() => useSignDetail("3.27", "ru"));
    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(result.current.sign).toEqual(detail);
    expect(result.current.error).toBeNull();
    expect(apiClient.apiGet).toHaveBeenCalledWith("signs/3.27?locale=ru");
  });

  it("does not request details before a sign is selected", async () => {
    const get = vi.spyOn(apiClient, "apiGet");

    const { result } = renderHook(() => useSignDetail(null, "uz-Latn"));

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(get).not.toHaveBeenCalled();
    expect(result.current.sign).toBeNull();
  });

  it("rejects a detail payload for a different sign code", async () => {
    vi.spyOn(apiClient, "apiGet").mockResolvedValue({
      code: "3.1",
      group_code: "prohibiting",
      name: "Kirish taqiqlangan",
      description: "Verified, but for a different sign.",
      image_url: null,
      question_ids: [],
    });

    const { result } = renderHook(() => useSignDetail("3.27", "uz-Latn"));
    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(result.current.sign).toBeNull();
    expect(result.current.error).toBe("unavailable");
  });
});
