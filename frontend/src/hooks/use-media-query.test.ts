import { describe, expect, it, vi, beforeEach, afterEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useIsCompact, useMediaQuery } from "./use-media-query";

type Listener = () => void;

function mockMatchMedia(initialMatches: boolean) {
  const listeners = new Set<Listener>();
  const mql = {
    matches: initialMatches,
    media: "",
    addEventListener: (_: string, cb: Listener) => {
      listeners.add(cb);
    },
    removeEventListener: (_: string, cb: Listener) => {
      listeners.delete(cb);
    },
  };
  vi.stubGlobal(
    "matchMedia",
    vi.fn(() => mql),
  );
  return {
    set(next: boolean) {
      mql.matches = next;
      listeners.forEach((cb) => cb());
    },
    listenerCount: () => listeners.size,
  };
}

describe("useMediaQuery", () => {
  beforeEach(() => {
    vi.unstubAllGlobals();
  });
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("returns the current match state", () => {
    mockMatchMedia(true);
    const { result } = renderHook(() => useMediaQuery("(max-width: 767px)"));
    expect(result.current).toBe(true);
  });

  it("re-renders when the media query changes", () => {
    const ctl = mockMatchMedia(false);
    const { result } = renderHook(() => useMediaQuery("(max-width: 767px)"));
    expect(result.current).toBe(false);
    act(() => ctl.set(true));
    expect(result.current).toBe(true);
  });

  it("removes its listener on unmount", () => {
    const ctl = mockMatchMedia(false);
    const { unmount } = renderHook(() => useMediaQuery("(max-width: 767px)"));
    expect(ctl.listenerCount()).toBe(1);
    unmount();
    expect(ctl.listenerCount()).toBe(0);
  });

  it("useIsCompact asks for the md breakpoint", () => {
    mockMatchMedia(true);
    const { result } = renderHook(() => useIsCompact());
    expect(result.current).toBe(true);
    expect(matchMedia).toHaveBeenCalledWith("(max-width: 767px)");
  });
});
