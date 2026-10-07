import { act, renderHook } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { useReportSessionRunning, useSessionRunning } from "./session-running";

describe("session-running store", () => {
  it("is false until a runner reports, and follows the runner's state", () => {
    const watcher = renderHook(() => useSessionRunning());
    expect(watcher.result.current).toBe(false);

    const runner = renderHook(({ running }) => useReportSessionRunning(running), {
      initialProps: { running: true },
    });
    expect(watcher.result.current).toBe(true);

    // The result screen: same route, nothing left to lose.
    act(() => runner.rerender({ running: false }));
    expect(watcher.result.current).toBe(false);

    act(() => runner.rerender({ running: true }));
    expect(watcher.result.current).toBe(true);
  });

  it("clears when the runner unmounts mid-attempt", () => {
    const watcher = renderHook(() => useSessionRunning());
    const runner = renderHook(() => useReportSessionRunning(true));
    expect(watcher.result.current).toBe(true);
    act(() => runner.unmount());
    expect(watcher.result.current).toBe(false);
  });

  it("stays on while any runner still reports running (overlapping route transition)", () => {
    const watcher = renderHook(() => useSessionRunning());
    const outgoing = renderHook(() => useReportSessionRunning(true));
    const incoming = renderHook(() => useReportSessionRunning(true));
    act(() => outgoing.unmount());
    expect(watcher.result.current).toBe(true);
    act(() => incoming.unmount());
    expect(watcher.result.current).toBe(false);
  });
});
