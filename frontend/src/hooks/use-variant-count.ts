"use client";

import { useEffect, useState } from "react";
import { apiGet } from "@/lib/api-client";
import {
  OFFICIAL_QUESTION_COUNT,
  OFFICIAL_TICKET_COUNT,
  OFFICIAL_TOPIC_COUNT,
} from "@/lib/content-counts";

type VariantListItem = { number: number; question_count?: number };
type CategoryListItem = { code: string };

export type CatalogCounts = {
  /** Numbered biletlar that actually exist in the catalog. */
  tickets: number;
  /** Questions those biletlar hold, summed from the same response. */
  questions: number;
  /** Topics (categories) the bank is filed under. */
  topics: number;
};

const FALLBACK: CatalogCounts = {
  tickets: OFFICIAL_TICKET_COUNT,
  questions: OFFICIAL_QUESTION_COUNT,
  topics: OFFICIAL_TOPIC_COUNT,
};

let catalogCountsCache: CatalogCounts | null = null;
let catalogCountsFetchedAt = 0;

export function clearCatalogCountsCacheForTests(): void {
  catalogCountsCache = null;
  catalogCountsFetchedAt = 0;
}

/**
 * Live catalog sizes, read from the catalog itself rather than quoted from
 * constants, so imported biletlar, questions and topics show up in the copy
 * without a code change. Falls back to the static counts until the responses
 * land, and keeps them for whichever request fails.
 */
export function useCatalogCounts(): CatalogCounts {
  const [counts, setCounts] = useState<CatalogCounts>(() => catalogCountsCache ?? FALLBACK);

  useEffect(() => {
    if (catalogCountsCache && Date.now() - catalogCountsFetchedAt < 30_000) {
      setCounts(catalogCountsCache);
      return;
    }

    let cancelled = false;

    void apiGet<VariantListItem[]>("variants")
      .then((list) => {
        if (cancelled || !Array.isArray(list) || list.length === 0) return;
        const questions = list.reduce((sum, v) => sum + (v.question_count ?? 0), 0);
        setCounts((prev) => {
          const next = {
            ...prev,
            tickets: list.length,
            questions: questions > 0 ? questions : prev.questions,
          };
          catalogCountsCache = next;
          catalogCountsFetchedAt = Date.now();
          return next;
        });
      })
      .catch(() => {
        // Keep the static fallback.
      });

    void apiGet<CategoryListItem[]>("categories")
      .then((list) => {
        if (cancelled || !Array.isArray(list) || list.length === 0) return;
        setCounts((prev) => {
          const next = { ...prev, topics: list.length };
          catalogCountsCache = next;
          catalogCountsFetchedAt = Date.now();
          return next;
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
