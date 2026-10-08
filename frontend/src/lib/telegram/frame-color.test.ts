import { act, renderHook } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { useReportTelegramFrameColor, useTelegramFrameColor } from "./frame-color";

describe("frame colour store", () => {
  it("follows the newest mounted report and clears on unmount", () => {
    const watcher = renderHook(() => useTelegramFrameColor());
    expect(watcher.result.current).toBeNull();
    const exam = renderHook(() => useReportTelegramFrameColor("#081320"));
    expect(watcher.result.current).toBe("#081320");
    act(() => exam.unmount());
    expect(watcher.result.current).toBeNull();
  });

  it("keeps the other screen's colour when the older one unmounts first", () => {
    const watcher = renderHook(() => useTelegramFrameColor());
    const outgoing = renderHook(() => useReportTelegramFrameColor("#111111"));
    const incoming = renderHook(() => useReportTelegramFrameColor("#222222"));
    act(() => outgoing.unmount());
    expect(watcher.result.current).toBe("#222222");
    act(() => incoming.unmount());
    expect(watcher.result.current).toBeNull();
  });

  it("ignores a null report", () => {
    const watcher = renderHook(() => useTelegramFrameColor());
    renderHook(() => useReportTelegramFrameColor(null));
    expect(watcher.result.current).toBeNull();
  });
});
