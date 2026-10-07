import { describe, expect, it } from "vitest";
import { isTabRoot, localeOf, needsClosingGuard } from "./routes";

describe("telegram routes", () => {
  it.each(["/uz-Latn/dashboard", "/ru/tickets", "/uz-Cyrl/practice", "/uz-Latn/exam", "/uz-Latn/profile", "/uz-Latn/tg", "/uz-Latn", "/"])(
    "%s is a tab root",
    (p) => expect(isTabRoot(p)).toBe(true)
  );
  it.each([
    "/uz-Latn/tickets/12",
    "/uz-Latn/signs",
    "/uz-Latn/session/abc",
    "/uz-Latn/premium",
    "/uz-Latn/arena",
    "/uz-Latn/practice/memorize/7",
    "/uz-Latn/checkout/pending",
  ])("%s shows Back", (p) => expect(isTabRoot(p)).toBe(false));
  it("tolerates a trailing slash on a tab root", () => {
    expect(isTabRoot("/uz-Latn/dashboard/")).toBe(true);
  });
  it("guards closing only inside a running test", () => {
    expect(needsClosingGuard("/uz-Latn/session/abc")).toBe(true);
    expect(needsClosingGuard("/uz-Latn/practice/memorize/7")).toBe(true);
    expect(needsClosingGuard("/uz-Latn/session/start")).toBe(false);
    expect(needsClosingGuard("/uz-Latn/session")).toBe(false);
    expect(needsClosingGuard("/uz-Latn/practice/memorize")).toBe(false);
    expect(needsClosingGuard("/uz-Latn/practice")).toBe(false);
    expect(needsClosingGuard("/uz-Latn/dashboard")).toBe(false);
  });
  it("reads a known locale from the path and falls back to the default", () => {
    expect(localeOf("/ru/signs")).toBe("ru");
    expect(localeOf("/uz-Cyrl")).toBe("uz-Cyrl");
    expect(localeOf("/xx/signs")).toBe("uz-Latn");
    expect(localeOf("/")).toBe("uz-Latn");
  });
});
