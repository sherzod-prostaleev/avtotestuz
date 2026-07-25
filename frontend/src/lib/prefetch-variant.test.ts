import { afterEach, describe, expect, it, vi } from "vitest";
import * as apiClient from "./api-client";
import { prefetchVariantDetail } from "./prefetch-variant";

afterEach(() => {
  vi.restoreAllMocks();
});

describe("prefetchVariantDetail", () => {
  it("requests variants/{n}?locale= for a positive ticket number", () => {
    const get = vi.spyOn(apiClient, "apiGet").mockResolvedValue({ number: 3, questions: [] });
    prefetchVariantDetail(3, "ru");
    expect(get).toHaveBeenCalledWith("variants/3?locale=ru");
  });

  it("ignores invalid numbers", () => {
    const get = vi.spyOn(apiClient, "apiGet").mockResolvedValue({});
    prefetchVariantDetail(0, "uz-Latn");
    prefetchVariantDetail(-1, "uz-Latn");
    prefetchVariantDetail("abc", "uz-Latn");
    expect(get).not.toHaveBeenCalled();
  });

  it("swallows fetch errors", () => {
    vi.spyOn(apiClient, "apiGet").mockRejectedValue(new Error("offline"));
    expect(() => prefetchVariantDetail(1, "uz-Latn")).not.toThrow();
  });
});
