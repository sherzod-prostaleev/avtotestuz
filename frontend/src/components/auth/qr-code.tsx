"use client";

import { useMemo } from "react";
import { encodeQr, qrSvgPath } from "@/lib/qr-code";

/**
 * A QR code as inline SVG. Always dark-on-white, whatever the theme: phone
 * cameras read inverted codes poorly, so the light quiet zone is part of it.
 */
export function QrCode({ value, label, className = "" }: { value: string; label: string; className?: string }) {
  const svg = useMemo(() => {
    try {
      return qrSvgPath(encodeQr(value));
    } catch {
      return null;
    }
  }, [value]);
  if (!svg) return null;
  return (
    <svg
      role="img"
      aria-label={label}
      viewBox={`0 0 ${svg.size} ${svg.size}`}
      shapeRendering="crispEdges"
      className={`rounded-xl bg-white ${className}`}
    >
      <rect width={svg.size} height={svg.size} fill="#ffffff" />
      <path d={svg.path} fill="#000000" />
    </svg>
  );
}
