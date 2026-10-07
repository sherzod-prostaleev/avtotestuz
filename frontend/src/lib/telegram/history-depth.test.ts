import { afterEach, describe, expect, it } from "vitest";
import { canGoBackInApp, installHistoryDepth } from "./history-depth";

let uninstall: (() => void) | null = null;
afterEach(() => {
  uninstall?.();
  uninstall = null;
  window.history.replaceState(null, "", "/");
});

describe("in-app history depth", () => {
  it("knows nothing about entries before it was installed", () => {
    window.history.replaceState({ __NA: true }, "", "/uz-Latn/signs");
    uninstall = installHistoryDepth();
    // The launch entry (e.g. /tg → replace → /signs) has nothing to go back to.
    expect(canGoBackInApp()).toBe(false);
    // ...and the stamp keeps Next's own state intact.
    expect(window.history.state.__NA).toBe(true);
  });

  it("counts pushes but not replaces", () => {
    uninstall = installHistoryDepth();
    window.history.replaceState({ __NA: true }, "", "/uz-Latn/dashboard");
    expect(canGoBackInApp()).toBe(false);
    window.history.pushState({ __NA: true }, "", "/uz-Latn/signs");
    expect(canGoBackInApp()).toBe(true);
    expect(window.history.state.__NA).toBe(true);
    window.history.replaceState({ __NA: true }, "", "/uz-Latn/signs?x=1");
    expect(canGoBackInApp()).toBe(true);
  });

  it("does not stamp once uninstalled", () => {
    uninstall = installHistoryDepth();
    uninstall();
    uninstall = null;
    window.history.pushState({ __NA: true }, "", "/uz-Latn/signs");
    expect(window.history.state).toEqual({ __NA: true });
  });

  it("tolerates a null state from third-party code", () => {
    uninstall = installHistoryDepth();
    window.history.pushState(null, "", "/uz-Latn/signs");
    expect(canGoBackInApp()).toBe(true);
  });
});
