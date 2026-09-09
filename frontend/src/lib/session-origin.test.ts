import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  SESSION_ORIGIN_KEY,
  isSessionOwnedPath,
  readSessionOrigin,
  rememberSessionOrigin,
} from "./session-origin";

describe("session origin", () => {
  beforeEach(() => {
    window.sessionStorage.clear();
    vi.restoreAllMocks();
  });

  it("remembers the hub a session was opened from", () => {
    rememberSessionOrigin("/uz-Latn/tickets");
    expect(readSessionOrigin()).toBe("/uz-Latn/tickets");
  });

  it("keeps the kiosk's own hub paths", () => {
    rememberSessionOrigin("/uz-Latn/station/tickets");
    expect(readSessionOrigin()).toBe("/uz-Latn/station/tickets");
  });

  // Recording the session itself would make "exit" point back into the screen
  // the learner is trying to leave.
  it.each([
    "/uz-Latn/session/abc",
    "/uz-Latn/session/start",
    "/uz-Latn/station/session/abc",
    "/uz-Latn/practice/memorize/signs",
    "/uz-Latn/station/practice/memorize/signs",
  ])("never records the session-owned path %s", (path) => {
    rememberSessionOrigin("/uz-Latn/tickets");
    rememberSessionOrigin(path);
    expect(isSessionOwnedPath(path)).toBe(true);
    expect(readSessionOrigin()).toBe("/uz-Latn/tickets");
  });

  it("refuses anything that is not a plain in-app path", () => {
    rememberSessionOrigin("//evil.example/uz-Latn");
    rememberSessionOrigin("https://evil.example");
    expect(readSessionOrigin()).toBeNull();
  });

  // A value can outlive the code that wrote it, so the read side validates too.
  it("drops a stored value that is no longer safe to navigate to", () => {
    window.sessionStorage.setItem(SESSION_ORIGIN_KEY, "//evil.example");
    expect(readSessionOrigin()).toBeNull();

    window.sessionStorage.setItem(SESSION_ORIGIN_KEY, "/uz-Latn/session/abc");
    expect(readSessionOrigin()).toBeNull();
  });

  it("degrades to no origin when the browser blocks site data", () => {
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
      throw new Error("blocked");
    });
    vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => {
      throw new Error("blocked");
    });

    expect(() => rememberSessionOrigin("/uz-Latn/tickets")).not.toThrow();
    expect(readSessionOrigin()).toBeNull();
  });

  it("returns null before anything has been recorded", () => {
    expect(readSessionOrigin()).toBeNull();
  });
});
