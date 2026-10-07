import { afterEach, describe, expect, it } from "vitest";
import { rememberTelegramLocale, resolveTelegramLocale, TG_LOCALE_KEY } from "./locale";
import { markTelegramMiniApp } from "./web-app";

afterEach(() => {
  localStorage.clear();
  sessionStorage.clear();
});

describe("resolveTelegramLocale", () => {
  it("sends a Russian-speaking Telegram user to ru on first open and remembers it", () => {
    expect(resolveTelegramLocale("uz-Latn", "ru")).toBe("ru");
    expect(localStorage.getItem(TG_LOCALE_KEY)).toBe("ru");
  });

  it("keeps the URL locale on first open for everyone else", () => {
    expect(resolveTelegramLocale("uz-Latn", "uz")).toBe("uz-Latn");
    localStorage.clear();
    expect(resolveTelegramLocale("uz-Cyrl", undefined)).toBe("uz-Cyrl");
  });

  it("prefers the remembered Mini App choice over language_code", () => {
    localStorage.setItem(TG_LOCALE_KEY, "uz-Cyrl");
    expect(resolveTelegramLocale("uz-Latn", "ru")).toBe("uz-Cyrl");
  });

  it("ignores a garbage stored value", () => {
    localStorage.setItem(TG_LOCALE_KEY, "en");
    expect(resolveTelegramLocale("uz-Latn", "uz")).toBe("uz-Latn");
    expect(localStorage.getItem(TG_LOCALE_KEY)).toBe("uz-Latn");
  });
});

describe("rememberTelegramLocale", () => {
  it("does nothing on the website", () => {
    rememberTelegramLocale("ru");
    expect(localStorage.getItem(TG_LOCALE_KEY)).toBeNull();
  });

  it("records an in-app language switch", () => {
    markTelegramMiniApp();
    rememberTelegramLocale("ru");
    expect(localStorage.getItem(TG_LOCALE_KEY)).toBe("ru");
  });
});
