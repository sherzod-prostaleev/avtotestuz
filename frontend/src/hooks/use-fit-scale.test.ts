import { describe, expect, it, vi, beforeEach, afterEach } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import { useFitScale } from "./use-fit-scale";

describe("useFitScale", () => {
  beforeEach(() => {
    vi.stubGlobal(
      "ResizeObserver",
      class {
        observe() {}
        unobserve() {}
        disconnect() {}
      }
    );
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("starts at scale 1", () => {
    const { result } = renderHook(() => useFitScale([]));
    expect(result.current.scale).toBe(1);
    expect(result.current.contentStyle).toEqual({});
  });

  it("exposes viewport and content refs for measurement", () => {
    const { result } = renderHook(() => useFitScale(["q1"]));
    expect(result.current.viewportRef).toBeDefined();
    expect(result.current.contentRef).toBeDefined();
  });

  it("applies transform style only when scale is below 1", async () => {
    const { result, rerender } = renderHook(
      ({ deps }) => useFitScale(deps),
      { initialProps: { deps: ["a"] as unknown[] } }
    );

    // Attach fake DOM nodes with overflow
    const viewport = document.createElement("div");
    const content = document.createElement("div");
    Object.defineProperty(viewport, "clientHeight", { value: 400, configurable: true });
    Object.defineProperty(content, "scrollHeight", { value: 800, configurable: true });

    // Stand in for the nodes React would have attached. React 19 types
    // RefObject.current as mutable, so this needs no escape hatch.
    result.current.viewportRef.current = viewport;
    result.current.contentRef.current = content;

    Object.defineProperty(window, "innerWidth", { value: 390, configurable: true });
    rerender({ deps: ["b"] });

    await waitFor(() => {
      expect(result.current.scale).toBeLessThan(1);
      expect(result.current.contentStyle.transform).toMatch(/scale\(/);
    });
  });
});
