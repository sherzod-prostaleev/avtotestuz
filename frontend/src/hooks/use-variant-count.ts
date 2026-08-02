"use client";

import { useEffect, useState } from "react";
import { apiGet } from "@/lib/api-client";
import { OFFICIAL_TICKET_COUNT } from "@/lib/content-counts";

type VariantListItem = { number: number };

/** Live numbered-bilet count from `GET /variants`, with a static fallback. */
export function useVariantCount(): number {
  const [count, setCount] = useState(OFFICIAL_TICKET_COUNT);

  useEffect(() => {
    let cancelled = false;
    apiGet<VariantListItem[]>("variants")
      .then((list) => {
        if (!cancelled && Array.isArray(list) && list.length > 0) {
          setCount(list.length);
        }
      })
      .catch(() => {
        // Keep OFFICIAL_TICKET_COUNT fallback.
      });
    return () => {
      cancelled = true;
    };
  }, []);

  return count;
}
