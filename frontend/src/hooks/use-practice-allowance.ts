"use client";

import { useCallback, useEffect, useState } from "react";
import { apiGet } from "@/lib/api-client";

export interface PracticeAllowance {
  unlimited: boolean;
  limit: number;
  used: number;
  remaining: number;
}

/**
 * Today's practice budget. The picker needs it before a session starts: the
 * server clamps an oversized request to whatever is left, so without this the
 * UI offers sizes it cannot deliver and the shortfall looks like a bug.
 */
export function usePracticeAllowance() {
  const [allowance, setAllowance] = useState<PracticeAllowance | null>(null);
  const [loading, setLoading] = useState(true);

  const refetch = useCallback(async () => {
    setLoading(true);
    try {
      const data = await apiGet<PracticeAllowance>("me/practice-allowance");
      setAllowance(data);
    } catch {
      // A missing budget must not block practice: fall back to no banner
      // rather than to an invented number.
      setAllowance(null);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void refetch();
  }, [refetch]);

  return { allowance, loading, refetch };
}
