import { describe, expect, it } from "vitest";
import { homeOrFallback, localizeCmsHref } from "./site-home";

describe("homeOrFallback", () => {
  it("prefers non-empty API values", () => {
    expect(homeOrFallback(" CMS ", "i18n")).toBe("CMS");
  });

  it("falls back when empty", () => {
    expect(homeOrFallback("", "i18n")).toBe("i18n");
    expect(homeOrFallback(undefined, "i18n")).toBe("i18n");
  });
});

describe("localizeCmsHref", () => {
  it("rewrites a hardcoded locale prefix to the visitor locale", () => {
    expect(localizeCmsHref("/uz-Latn/login", "ru")).toBe("/ru/login");
    expect(localizeCmsHref("/uz-Cyrl", "uz-Latn")).toBe("/uz-Latn");
  });

  it("leaves non-locale paths alone", () => {
    expect(localizeCmsHref("/login", "ru")).toBe("/login");
    expect(localizeCmsHref("https://t.me/DriverGo", "ru")).toBe("https://t.me/DriverGo");
  });
});
