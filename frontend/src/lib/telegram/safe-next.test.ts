import { describe, expect, it } from "vitest";
import { carryNextQuery, miniAppNext, safeNextPath } from "./safe-next";

describe("safeNextPath", () => {
  it.each([
    [null, "/uz-Latn/dashboard"],
    ["/uz-Latn/tickets", "/uz-Latn/tickets"],
    ["/uz-Latn/session/abc?x=1", "/uz-Latn/session/abc?x=1"],
    ["//evil.example", "/uz-Latn/dashboard"],
    ["/uz-Latn//evil.example", "/uz-Latn/dashboard"],
    ["http://x/uz-Latn/foo", "/uz-Latn/dashboard"],
    ["uz-Latn/foo", "/uz-Latn/dashboard"],
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

describe("miniAppNext / carryNextQuery", () => {
  const webApp = {};
  it("passes next through only with a live WebApp", () => {
    expect(miniAppNext(webApp, "/uz-Latn/signs")).toBe("/uz-Latn/signs");
    expect(miniAppNext(null, "/uz-Latn/signs")).toBeNull();
  });

  it("carries next exactly when the redirect would honour it", () => {
    expect(carryNextQuery(webApp, "/uz-Latn/signs", "uz-Latn")).toBe("?next=%2Fuz-Latn%2Fsigns");
    expect(carryNextQuery(null, "/uz-Latn/signs", "uz-Latn")).toBe("");
    expect(carryNextQuery(webApp, null, "uz-Latn")).toBe("");
    // Targets that fall back to the dashboard anyway are not worth a query.
    expect(carryNextQuery(webApp, "https://evil.example/x", "uz-Latn")).toBe("");
    expect(carryNextQuery(webApp, "/uz-Latn/tg", "uz-Latn")).toBe("");
  });
});
