import { apiGet } from "@/lib/api-client";

/**
 * Prefetch a ticket/bilet variant detail so the service worker can cache the
 * grading-neutral question payload for later offline reads (U-39).
 * Best-effort — never blocks navigation; failures are ignored.
 */
export function prefetchVariantDetail(variantNumber: number | string, locale: string): void {
  const n = Number(variantNumber);
  if (!Number.isInteger(n) || n < 1) return;
  const loc = (locale || "uz-Latn").trim();
  if (!loc) return;
  void apiGet(`variants/${n}?locale=${encodeURIComponent(loc)}`).catch(() => {
    /* offline / unauth — SW will still serve a prior cache hit if any */
  });
}
