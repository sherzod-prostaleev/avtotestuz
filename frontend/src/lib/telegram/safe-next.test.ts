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
    ["/uz-Latn/x/../tg?next=%2Fuz-Latn%2Ftickets", "/uz-Latn/dashboard"],
    ["/uz-Latn/a/./b", "/uz-Latn/a/b"],
    ["/uz-Latn/%2e%2e/tg", "/uz-Latn/dashboard"],
    ["/uz-Latn/x/../../evil", "/uz-Latn/dashboard"],
  ])("%s → %s", (raw, want) => {
    expect(safeNextPath(raw, "uz-Latn")).toBe(want);
  });
});
