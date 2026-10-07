import { describe, expect, it } from "vitest";
import { safeNextPath } from "./safe-next";

describe("safeNextPath", () => {
  it.each([
    [null, "/uz-Latn/dashboard"],
    ["/uz-Latn/tickets", "/uz-Latn/tickets"],
    ["/uz-Latn/session/abc?x=1", "/uz-Latn/session/abc?x=1"],
    ["//evil.example", "/uz-Latn/dashboard"],
    ["/uz-Latn//evil.example", "/uz-Latn/dashboard"],
    ["https://evil.example", "/uz-Latn/dashboard"],
    ["/ru/tickets", "/uz-Latn/dashboard"],
    ["/uz-Latn/tg", "/uz-Latn/dashboard"],
    ["/uz-Latn\\evil", "/uz-Latn/dashboard"],
  ])("%s → %s", (raw, want) => {
    expect(safeNextPath(raw, "uz-Latn")).toBe(want);
  });
});
