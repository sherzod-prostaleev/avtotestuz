import { describe, expect, it } from "vitest";
import { homeOrFallback } from "./site-home";

describe("homeOrFallback", () => {
  it("prefers non-empty API values", () => {
    expect(homeOrFallback(" CMS ", "i18n")).toBe("CMS");
  });

  it("falls back when empty", () => {
    expect(homeOrFallback("", "i18n")).toBe("i18n");
    expect(homeOrFallback(undefined, "i18n")).toBe("i18n");
  });
});
