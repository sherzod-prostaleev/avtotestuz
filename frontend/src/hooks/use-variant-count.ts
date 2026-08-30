"use client";

import { useEffect, useState } from "react";
import { apiGet } from "@/lib/api-client";
import { OFFICIAL_QUESTION_COUNT, OFFICIAL_TICKET_COUNT } from "@/lib/content-counts";

type VariantListItem = { number: number; question_count?: number };

export type CatalogCounts = {
  /** Numbered biletlar that actually exist in the catalog. */
  tickets: number;
  /** Questions those biletlar hold, summed from the same response. */
  questions: number;
};

const FALLBACK: CatalogCounts = {
  tickets: OFFICIAL_TICKET_COUNT,
  questions: OFFICIAL_QUESTION_COUNT,
};

/**
 * Live bilet/question counts, read from the catalog itself (`GET /variants`)
 * rather than quoted from a constant, so an imported bilet shows up in the copy
 * without a code change. Falls back to the static counts until the response
 * lands, and keeps them if it never does.
 */
export function useCatalogCounts(): CatalogCounts {
  const [counts, setCounts] = useState<CatalogCounts>(FALLBACK);

  useEffect(() => {
    let cancelled = false;
    apiGet<VariantListItem[]>("variants")
      .then((list) => {
        if (cancelled || !Array.isArray(list) || list.length === 0) return;
        const questions = list.reduce((sum, v) => sum + (v.question_count ?? 0), 0);
        setCounts({
          tickets: list.length,
          questions: questions > 0 ? questions : FALLBACK.questions,
        });
      })
      .catch(() => {
        // Keep the static fallback.
      });
    return () => {
      cancelled = true;
    };
  }, []);

  return counts;
}

/** Live numbered-bilet count from `GET /variants`, with a static fallback. */
export function useVariantCount(): number {
  return useCatalogCounts().tickets;
}
