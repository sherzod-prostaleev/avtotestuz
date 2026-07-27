"use client";

import { useLayoutEffect, useRef, useState, type CSSProperties, type RefObject } from "react";

const MIN_SCALE = 0.62;
const LG_BREAKPOINT = 1024;

export interface FitScaleResult {
  viewportRef: RefObject<HTMLDivElement>;
  contentRef: RefObject<HTMLDivElement>;
  scale: number;
  contentStyle: CSSProperties;
}

/**
 * Scales content down (never above 1) so it fits the handed viewport height.
 *
 * Pair with a viewport that is `h-full min-h-0 overflow-hidden`. Transform does not
 * change layout size, so the viewport clips; visually the block fits with no scroll.
 * Desktop (lg+) always stays at scale 1 — two-column layout owns height there.
 */
export function useFitScale(deps: ReadonlyArray<unknown> = []): FitScaleResult {
  const viewportRef = useRef<HTMLDivElement>(null!);
  const contentRef = useRef<HTMLDivElement>(null!);
  const [scale, setScale] = useState(1);

  useLayoutEffect(() => {
    const viewport = viewportRef.current;
    const content = contentRef.current;
    if (!viewport || !content) return;

    let frame = 0;

    const measure = () => {
      cancelAnimationFrame(frame);
      frame = requestAnimationFrame(() => {
        if (typeof window !== "undefined" && window.innerWidth >= LG_BREAKPOINT) {
          setScale(1);
          return;
        }

        const prevTransform = content.style.transform;
        const prevOrigin = content.style.transformOrigin;
        content.style.transform = "scale(1)";
        content.style.transformOrigin = "top center";

        const available = viewport.clientHeight;
        const needed = content.scrollHeight;

        content.style.transform = prevTransform;
        content.style.transformOrigin = prevOrigin;

        if (available <= 0 || needed <= 0) {
          setScale(1);
          return;
        }

        const next = needed > available + 1 ? Math.max(MIN_SCALE, available / needed) : 1;
        setScale((prev) => (Math.abs(prev - next) < 0.008 ? prev : next));
      });
    };

    measure();

    const ro = typeof ResizeObserver !== "undefined" ? new ResizeObserver(measure) : null;
    ro?.observe(viewport);
    ro?.observe(content);
    window.addEventListener("resize", measure);
    window.addEventListener("orientationchange", measure);

    return () => {
      cancelAnimationFrame(frame);
      ro?.disconnect();
      window.removeEventListener("resize", measure);
      window.removeEventListener("orientationchange", measure);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps -- explicit layout deps from caller
  }, deps);

  const contentStyle: CSSProperties =
    scale < 1
      ? {
          transform: `scale(${scale})`,
          transformOrigin: "top center",
        }
      : {};

  return { viewportRef, contentRef, scale, contentStyle };
}
