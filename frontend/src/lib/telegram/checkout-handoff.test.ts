import { afterEach, describe, expect, it, vi } from "vitest";
import {
  CHECKOUT_URL_KEY,
  CHECKOUT_URL_MAX_AGE_MS,
  forgetCheckoutUrl,
  readCheckoutUrl,
  rememberCheckoutUrl,
} from "./checkout-handoff";

const URL_OK = "https://checkout.paycom.uz/abc";

afterEach(() => {
  vi.useRealTimers();
  sessionStorage.clear();
});

describe("checkout hand-off storage", () => {
  it("round-trips a fresh http(s) URL", () => {
    rememberCheckoutUrl(URL_OK);
    expect(readCheckoutUrl()).toBe(URL_OK);
  });

  it("discards an entry older than the max age and removes it", () => {
    vi.useFakeTimers();
    rememberCheckoutUrl(URL_OK);
    vi.advanceTimersByTime(CHECKOUT_URL_MAX_AGE_MS - 1);
    expect(readCheckoutUrl()).toBe(URL_OK);
    vi.advanceTimersByTime(2);
    expect(readCheckoutUrl()).toBeNull();
    expect(sessionStorage.getItem(CHECKOUT_URL_KEY)).toBeNull();
  });

  it.each([
    ["a bare URL string", URL_OK],
    ["malformed JSON", "{"],
    ["a missing timestamp", JSON.stringify({ url: URL_OK })],
    ["a non-http URL", JSON.stringify({ url: "javascript:alert(1)", at: Date.now() })],
  ])("ignores %s", (_name, raw) => {
    sessionStorage.setItem(CHECKOUT_URL_KEY, raw);
    expect(readCheckoutUrl()).toBeNull();
  });

  it("forgets the stored URL", () => {
    rememberCheckoutUrl(URL_OK);
    forgetCheckoutUrl();
    expect(readCheckoutUrl()).toBeNull();
  });
});
