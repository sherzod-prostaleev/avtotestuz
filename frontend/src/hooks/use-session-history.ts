import { useCallback, useEffect, useState } from "react";
import { apiGet, ApiError } from "@/lib/api-client";

export interface SessionSummary {
  id: string;
  mode: "variant" | "exam" | "practice" | "mistakes" | "grand_mock" | "review" | "placement";
  status: "in_progress" | "passed" | "failed" | "abandoned";
  score?: number;
  total: number;
  started_at: string;
  finished_at?: string;
}

let sessionHistoryCache: SessionSummary[] | null = null;

export function clearSessionHistoryCache(): void {
  sessionHistoryCache = null;
}

export function useSessionHistory(limit = 20) {
  const [sessions, setSessions] = useState<SessionSummary[]>(() => sessionHistoryCache ?? []);
  const [loading, setLoading] = useState(() => sessionHistoryCache === null);
  const [error, setError] = useState<string | null>(null);

  const fetchSessions = useCallback(async () => {
    if (!sessionHistoryCache) {
      setLoading(true);
    }
    setError(null);
    try {
      const data = await apiGet<SessionSummary[]>(`me/sessions?limit=${limit}`);
      sessionHistoryCache = data;
      setSessions(data);
    } catch (requestError) {
      setError(requestError instanceof ApiError ? requestError.message : "Failed to load session history");
    } finally {
      setLoading(false);
    }
  }, [limit]);

  useEffect(() => {
    void fetchSessions();
  }, [fetchSessions]);

  return { sessions, loading, error, refetch: fetchSessions };
}

